// Package network is generated by Handlergenerator tooling
// OpenPcap will open up a pcap and output all network packets to the next processor
package network

import (
	"context"
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/percybolmer/workflow/metric"
	"github.com/percybolmer/workflow/payload"
	"github.com/percybolmer/workflow/property"
	"github.com/percybolmer/workflow/pubsub"
	"github.com/percybolmer/workflow/register"
)

// OpenPcap is used to open up a PCAP and Read the packets from the pcap.
type OpenPcap struct {
	// Cfg is values needed to properly run the Handle func
	Cfg *property.Configuration `json:"configs" yaml:"configs"`
	// Name is sort of like an ID used to load data back should be the same that is used to register the Handler
	Name string `json:"handler_name" yaml:"handler_name"`
	// bpf is used to apply a bpf filter
	bpf string
	// subscriptionless should be set to true if this Handler does not need any input payloads to function
	subscriptionless bool
	errChan          chan error
	metrics          metric.Provider
	metricPrefix     string
	// MetricPayloadOut is how many payloads the processor has outputted
	MetricPayloadOut string
	// MetricPayloadIn is how many payloads the processor has inputted
	MetricPayloadIn string
}

func init() {
	register.Register("OpenPcap", NewOpenPcapHandler())
}

// NewOpenPcapHandler generates a new OpenPcap Handler
func NewOpenPcapHandler() *OpenPcap {
	act := &OpenPcap{
		Cfg: &property.Configuration{
			Properties: make([]*property.Property, 0),
		},
		Name:    "OpenPcap",
		errChan: make(chan error, 1000),
	}
	act.Cfg.AddProperty("bpf", "A bpf filter to be used on the input pcap", false)
	return act
}

// GetHandlerName should return the name of the handler that was used in register
func (a *OpenPcap) GetHandlerName() string {
	return a.Name
}

// Handle is used to open a pcap and output all network packets
func (a *OpenPcap) Handle(ctx context.Context, input payload.Payload, topics ...string) error {
	a.metrics.IncrementMetric(a.MetricPayloadIn, 1)
	path := string(input.GetPayload())
	file, err := pcap.OpenOffline(path)
	if err != nil {
		return err
	}
	defer func() {
		file.Close()
	}()

	// apply filter if not empty
	if a.bpf != "" {
		err = file.SetBPFFilter(a.bpf)
		if err != nil {
			return err
		}
	}

	packets := gopacket.NewPacketSource(file, file.LinkType())

	var payloads []payload.Payload

	for packet := range packets.Packets() {
		payloads = append(payloads, &Payload{
			Source:  "OpenPcap",
			Payload: packet,
		})
	}

	a.metrics.IncrementMetric(a.MetricPayloadOut, float64(len(payloads)))
	errs := pubsub.PublishTopics(topics, payloads...)
	if errs != nil {
		for _, err := range errs {
			a.errChan <- err
		}
	}

	return nil
}

// ValidateConfiguration is used to see that all needed configurations are assigned before starting
func (a *OpenPcap) ValidateConfiguration() (bool, []string) {
	// Check if Cfgs are there as needed
	valid, miss := a.Cfg.ValidateProperties()
	if !valid {
		return valid, miss
	}

	bpfProp := a.Cfg.GetProperty("bpf")
	if bpfProp != nil && bpfProp.Value != nil {
		a.bpf = bpfProp.String()
	}
	return true, nil
}

// GetConfiguration will return the CFG for the Handler
func (a *OpenPcap) GetConfiguration() *property.Configuration {
	return a.Cfg
}

// Subscriptionless will return true/false if the Handler is genereating payloads itself
func (a *OpenPcap) Subscriptionless() bool {
	return a.subscriptionless
}

// GetErrorChannel will return a channel that the Handler can output eventual errors onto
func (a *OpenPcap) GetErrorChannel() chan error {
	return a.errChan
}

// SetMetricProvider is used to change what metrics provider is used by the handler
func (a *OpenPcap) SetMetricProvider(p metric.Provider, prefix string) error {
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
