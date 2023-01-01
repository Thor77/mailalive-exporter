package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/emersion/go-imap"
	sortthread "github.com/emersion/go-imap-sortthread"
	"github.com/emersion/go-imap/client"
	"github.com/jellydator/ttlcache/v3"
)

type status struct {
	Timestamp float64
	Delay     float64
}

func (s status) ByName(name string) float64 {
	if name == "timestamp" {
		return s.Timestamp
	} else if name == "delay" {
		return s.Delay
	}
	return 0
}

var config Config
var cache *ttlcache.Cache[string, status]
var cacheKey = "status"
var subjectPrefix = "Alive check "
var lock sync.Mutex

var (
	mailgunErrorMetric = metrics.NewCounter(formatMetric(`errors_total{error="mailgun"}`))
	imapErrorMetric    = metrics.NewCounter(formatMetric(`errors_total{error="imap"}`))
	deletionsMetric    = metrics.NewCounter(formatMetric("deletions_total"))
)

func formatMetric(format string, params ...any) string {
	return fmt.Sprintf("mailalive_"+format, params...)
}

func sendMailgunMail() error {
	apiURL := fmt.Sprintf("https://api.eu.mailgun.net/v3/%s/messages", config.Mailgun.Domain)

	values := url.Values{}
	values.Add("from", fmt.Sprintf("Mailgun <mailgun@%s>", config.Mailgun.Domain))
	values.Add("to", config.Mailgun.To)
	values.Add("subject", fmt.Sprintf("%s%d", subjectPrefix, time.Now().UTC().Unix()))
	values.Add("text", "This message is used to check end-to-end mail delivery.")
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth("api", config.Mailgun.APIKey)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("error sending mail: %s", resp.Status)
	}
	return nil
}

func fetchAliveStatus() (status, error) {
	s := status{}

	c, err := client.DialTLS(config.IMAP.Addr, nil)
	if err != nil {
		return s, err
	}
	defer c.Logout()

	if err := c.Login(config.IMAP.Username, config.IMAP.Password); err != nil {
		return s, err
	}

	mbox, err := c.Select("INBOX", false)
	if err != nil {
		return s, err
	}

	// fetch message UIDs reverse sorted by arrival
	sc := sortthread.NewSortClient(c)
	sortCriteria := []sortthread.SortCriterion{
		{Field: sortthread.SortArrival, Reverse: true},
	}
	searchCriteria := imap.NewSearchCriteria()
	uids, err := sc.UidSort(sortCriteria, searchCriteria)
	if err != nil {
		return s, err
	}

	if len(uids) == 0 {
		return s, errors.New("no messages found")
	} else if len(uids) > 1 {
		log.Println("deleting messages")
		// mark all but first message for deletion
		flagSeqSet := new(imap.SeqSet)
		flagSeqSet.AddNum(uids[1:]...)
		if err := c.UidStore(flagSeqSet, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.DeletedFlag}, nil); err != nil {
			return s, err
		}

		if err := c.Expunge(nil); err != nil {
			return s, err
		}

		deletionsMetric.Add(len(flagSeqSet.Set))
	}

	// fetch first message
	targetMsgSeqSet := new(imap.SeqSet)
	targetMsgSeqSet.AddNum(uids[0])
	messages := make(chan *imap.Message, mbox.Messages)
	if err := c.UidFetch(targetMsgSeqSet, []imap.FetchItem{imap.FetchEnvelope, imap.FetchInternalDate}, messages); err != nil {
		return s, err
	}
	if len(messages) == 0 {
		return s, errors.New("couldn't fetch message")
	}
	message := <-messages

	// parse timestamp from subject
	subject := strings.TrimPrefix(message.Envelope.Subject, subjectPrefix)
	subjectParsed, err := strconv.ParseInt(subject, 10, 64)
	if err != nil {
		return s, err
	}
	subjectTime := time.Unix(int64(subjectParsed), 0)
	arrivalTime := message.InternalDate
	delay := arrivalTime.Sub(subjectTime)
	s.Delay = delay.Seconds()
	s.Timestamp = float64(subjectTime.Unix())

	return s, nil
}

func fetchAliveStatusCache(metric string) float64 {
	lock.Lock()
	defer lock.Unlock()
	if item := cache.Get(cacheKey); item != nil {
		return item.Value().ByName(metric)
	} else {
		// insert value into cache
		status, err := fetchAliveStatus()
		if err != nil {
			log.Printf("error fetching alive status: %v\n", err)
			imapErrorMetric.Inc()
		} else {
			cache.Set(cacheKey, status, ttlcache.NoTTL)
			return status.ByName(metric)
		}
	}
	return 0
}

func main() {
	http.HandleFunc("/metrics", func(w http.ResponseWriter, req *http.Request) {
		metrics.WritePrometheus(w, true)
	})

	// parse config
	var configPath string
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	} else {
		configPath = "config.toml"
	}
	var err error
	config, err = ParseConfig(configPath)
	if err != nil {
		log.Fatalf("error parsing config: %v\n", err)
	}

	// setup cache
	cache = ttlcache.New[string, status]()
	go func() {
		for {
			log.Println("expiring cache")
			cache.DeleteAll()
			time.Sleep(5 * time.Minute)
		}
	}()

	// initialize metrics
	metrics.NewGauge(formatMetric("message_delay"), func() float64 { return fetchAliveStatusCache("delay") })
	metrics.NewGauge(formatMetric("message_timestamp"), func() float64 { return fetchAliveStatusCache("timestamp") })

	go func() {
		for {
			log.Println("sending message")
			if err := sendMailgunMail(); err != nil {
				log.Printf("error sending mailgun request: %v\n", err)
				mailgunErrorMetric.Inc()
			}
			time.Sleep(1 * time.Hour)
		}
	}()

	log.Printf("Listening on %s\n", config.Addr)
	http.ListenAndServe(config.Addr, nil)
}
