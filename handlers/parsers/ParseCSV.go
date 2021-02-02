// Package parsers is generated by Handlergenerator tooling
// Make sure to insert real Description here
package parsers

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/percybolmer/go4data/handlers"
	"github.com/percybolmer/go4data/metric"
	"github.com/percybolmer/go4data/payload"
	"github.com/percybolmer/go4data/property"
	"github.com/percybolmer/go4data/pubsub"
	"github.com/percybolmer/go4data/register"
)

var (
	// DefaultDelimiter is the delimiter used if nothing is set
	DefaultDelimiter = ","
	// DefaultHeaderLength is the headerlength to use if nothing else is set
	DefaultHeaderLength = 1
	// DefaultSkipRows is the default rows to skip if nothing is set
	DefaultSkipRows = 0

	//ErrNotCsv is triggered when the input file is not proper csv
	ErrNotCsv error = errors.New("this is not a proper csv file")
	//ErrHeaderMismatch is triggered when header is longer than CSV records
	ErrHeaderMismatch error = errors.New("the header is not the same size as the records")
)

// ParseCSV is used to parse CSV files, expects whole payloads
type ParseCSV struct {
	// Cfg is values needed to properly run the Handle func
	Cfg  *property.Configuration `json:"configs" yaml:"configs"`
	Name string                  `json:"handler_name" yaml:"handler_name"`
	// delimiter is the character to use for delimiting
	delimiter string
	// headerlength is a int that is used for the base of the header, some files has duplicate headers etc
	headerlength int
	// skiprows is used to skip some rows if there is excess rows in the file
	skiprows int

	subscriptionless bool
	errChan          chan error

	metrics      metric.Provider
	metricPrefix string
	// MetricPayloadOut is how many payloads the processor has outputted
	MetricPayloadOut string
	// MetricPayloadIn is how many payloads the processor has inputted
	MetricPayloadIn string
}

func init() {
	register.Register("ParseCSV", NewParseCSVHandler)
}

// NewParseCSVHandler generates a new ParseCSV Handler
func NewParseCSVHandler() handlers.Handler {
	act := &ParseCSV{
		Cfg: &property.Configuration{
			Properties: make([]*property.Property, 0),
		},
		Name:         "ParseCSV",
		delimiter:    DefaultDelimiter,
		headerlength: DefaultHeaderLength,
		skiprows:     DefaultSkipRows,
		errChan:      make(chan error, 1000),
	}
	act.Cfg.AddProperty("delimiter", "The character or string to use as a Delimiter", false)
	act.Cfg.AddProperty("headerlength", "How many rows the header is", false)
	act.Cfg.AddProperty("skiprows", "How many rows will be skipped in each file before starting to process", false)
	return act
}

// GetHandlerName is used to retrun a unqiue string name
func (a *ParseCSV) GetHandlerName() string {
	return a.Name
}

// Handle will go through a CSV payload and output all the CSV rows
func (a ParseCSV) Handle(ctx context.Context, input payload.Payload, topics ...string) error {
	a.metrics.IncrementMetric(a.MetricPayloadIn, 1)
	buf := bytes.NewBuffer(input.GetPayload())

	scanner := bufio.NewScanner(buf)
	// Index keeps track of Line index in payload
	var index int

	header := make([]string, 0)
	headerRow := ""
	result := make([]payload.Payload, 0)

	for scanner.Scan() {
		line := scanner.Text()
		// Handle skiprows
		if index < a.skiprows {
			index++
			continue
		}

		// Handle Header row
		values := strings.Split(line, a.delimiter)
		if len(values) <= 1 {
			return ErrNotCsv
		}

		// Handle Unique Cases of header rows longer than 1 line
		if index < (a.skiprows + a.headerlength) {
			header = append(header, values...)
			headerRow = strings.Join(header, a.delimiter)
			index++
			continue
		}

		// Make sure header is no longer than current values
		if len(header) != len(values) {
			return ErrHeaderMismatch
		}
		// Handle the CSV ROW as a Map of string, should this be interface?
		newRow := payload.NewCsvPayload(headerRow, line, a.delimiter, nil)
		result = append(result, newRow)
	}

	// Publish rows
	a.metrics.IncrementMetric(a.MetricPayloadOut, float64(len(result)))
	errs := pubsub.PublishTopics(topics, result...)
	if errs != nil {
		for _, err := range errs {
			a.errChan <- err
		}
	}
	return nil
}

// ValidateConfiguration is used to see that all needed configurations are assigned before starting
func (a *ParseCSV) ValidateConfiguration() (bool, []string) {
	// Check if Cfgs are there as needed
	delimiterProp := a.Cfg.GetProperty("delimiter")
	headerProp := a.Cfg.GetProperty("headerlength")
	skiprowProp := a.Cfg.GetProperty("skiprows")

	missing := make([]string, 0)

	if delimiterProp == nil || headerProp == nil || skiprowProp == nil {
		// Abit lazy here and just return all 3 props
		missing = append(missing, "delimiter", "headerlength", "skiprows")
		return false, missing
	}

	if delimiterProp.Value != nil {
		a.delimiter = delimiterProp.String()
	}
	if headerProp.Value != nil {
		headerlength, err := headerProp.Int()
		if err != nil {
			return false, nil
		}
		a.headerlength = headerlength
	}
	if skiprowProp.Value != nil {
		skiprow, err := skiprowProp.Int()
		if err != nil {
			return false, nil
		}

		a.skiprows = skiprow
	}
	return true, nil
}

// GetConfiguration will return the CFG for the Handler
func (a *ParseCSV) GetConfiguration() *property.Configuration {
	return a.Cfg
}

// Subscriptionless will return true/false if the Handler is genereating payloads itself
func (a *ParseCSV) Subscriptionless() bool {
	return a.subscriptionless
}

// GetErrorChannel will return a channel that the Handler can output eventual errors onto
func (a *ParseCSV) GetErrorChannel() chan error {
	return a.errChan
}

// SetMetricProvider is used to change what metrics provider is used by the handler
func (a *ParseCSV) SetMetricProvider(p metric.Provider, prefix string) error {
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
