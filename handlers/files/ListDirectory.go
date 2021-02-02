// Package files is generated by Handlergenerator tooling
// Make sure to insert real Description here
package files

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/percybolmer/go4data/handlers"
	"github.com/percybolmer/go4data/metric"
	"github.com/percybolmer/go4data/payload"
	"github.com/percybolmer/go4data/property"
	"github.com/percybolmer/go4data/pubsub"
	"github.com/percybolmer/go4data/register"
)

// ListDirectory is used to list all FILES in a given path
type ListDirectory struct {
	// Cfg is values needed to properly run the Handle func
	Cfg        *property.Configuration `json:"configs" yaml:"configs"`
	Name       string                  `json:"handler" yaml:"handler_name"`
	path       string
	buffertime int64
	found      map[string]int64
	sync.Mutex `json:"-" yaml:"-"`

	subscriptionless bool
	errChan          chan error

	metrics      metric.Provider
	metricPrefix string

	// MetricPayloadOut is how many payloads the processor has outputted
	MetricPayloadOut string
	// MetricPayloadIn is how many payloads the processor has inputted
	MetricPayloadIn string
}

var (
	// DefaultBufferTime is how long in seconds a file should be fulfillremembered
	DefaultBufferTime int64 = 3600
)

func init() {
	register.Register("ListDirectory", NewListDirectoryHandler)
}

// NewListDirectoryHandler generates a new ListDirectory Handler
func NewListDirectoryHandler() handlers.Handler {
	act := &ListDirectory{
		Cfg: &property.Configuration{
			Properties: make([]*property.Property, 0),
		},
		Name:             "ListDirectory",
		found:            make(map[string]int64),
		subscriptionless: true,
		errChan:          make(chan error, 1000),

		metrics: metric.NewPrometheusProvider(),
	}

	act.Cfg.AddProperty("path", "the path to search for", true)
	act.Cfg.AddProperty("buffertime", "the time in seconds for how long a found file should be fulfillremembered and not relisted", false)

	return act
}

// GetHandlerName is used to retrun a unqiue string name
func (a *ListDirectory) GetHandlerName() string {
	return a.Name
}

// Handle is used to list all files in a direcory
func (a *ListDirectory) Handle(ctx context.Context, p payload.Payload, topics ...string) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			payloads, err := a.ListDirectory()
			if err != nil {
				a.errChan <- err
				continue
			}
			if len(payloads) != 0 {
				a.metrics.IncrementMetric(a.MetricPayloadOut, float64(len(payloads)))
				errs := pubsub.PublishTopics(topics, payloads...)
				for _, err := range errs {
					fmt.Println(err)
					a.errChan <- err
				}

			}
		}
	}
}

// ListDirectory will do all the main work, list directory or return error
func (a *ListDirectory) ListDirectory() ([]payload.Payload, error) {
	files, err := ioutil.ReadDir(a.path)
	if err != nil {
		return nil, err
	}
	a.Lock()
	for k, v := range a.found {
		if time.Now().Unix()-v > a.buffertime {
			delete(a.found, k) // If the item is older than given time setting, delete it from buffer
		}
	}
	a.Unlock()
	var outputPayloads []payload.Payload
	for _, f := range files {
		if f.IsDir() == false {
			file := filepath.Base(f.Name())
			var filepath string
			if strings.HasSuffix(a.path, "/") {
				filepath = fmt.Sprintf("%s%s", a.path, file)
			} else {
				filepath = fmt.Sprintf("%s/%s", a.path, file)
			}
			if _, ok := a.found[filepath]; !ok {
				outputPayloads = append(outputPayloads, payload.NewBasePayload([]byte(filepath), "ListDirectory", nil))
				a.found[filepath] = time.Now().Unix()
			}
		}
	}
	return outputPayloads, nil
}

// ValidateConfiguration is used to see that all needed configurations are assigned before starting
func (a *ListDirectory) ValidateConfiguration() (bool, []string) {
	// Check if Cfgs are there as needed
	// Needs a Directory to monitor
	pathProp := a.Cfg.GetProperty("path")
	missing := make([]string, 0)
	if pathProp == nil {
		missing = append(missing, "path")
		return false, missing
	}
	bufferProp := a.Cfg.GetProperty("buffertime")
	if bufferProp.Value == nil {
		a.buffertime = DefaultBufferTime
	} else {
		value, err := bufferProp.Int64()
		if err != nil {
			missing = append(missing, "buffertime")
			return false, missing
		}
		a.buffertime = value
	}

	a.path = pathProp.String()
	return true, nil
}

// GetConfiguration will return the CFG for the Handler
func (a *ListDirectory) GetConfiguration() *property.Configuration {
	return a.Cfg
}

// Subscriptionless is used to send out true
func (a *ListDirectory) Subscriptionless() bool {
	return a.subscriptionless
}

// GetErrorChannel will return a channel that the Handler can output eventual errors onto
func (a *ListDirectory) GetErrorChannel() chan error {
	return a.errChan
}

// SetMetricProvider is used to change what metrics provider is used by the handler
func (a *ListDirectory) SetMetricProvider(p metric.Provider, prefix string) error {
	a.metrics = p
	a.metricPrefix = prefix

	a.MetricPayloadIn = fmt.Sprintf("%s_payloads_in", prefix)
	a.MetricPayloadOut = fmt.Sprintf("%s_payloads_out", prefix)
	err := a.metrics.AddMetric(&metric.Metric{
		Name:        a.MetricPayloadOut,
		Description: "keeps track of how many payloads the handler has outputted",
	})
	if err != nil {
		return err
	}
	err = a.metrics.AddMetric(&metric.Metric{
		Name:        a.MetricPayloadIn,
		Description: "keeps track of how many payloads the handler has ingested",
	})
	return err
}
