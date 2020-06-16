// package fileprocessors is generated by generate-processor tooling
// Make sure to insert real Description here
package fileprocessors


import (
    "context"
    "fmt"
    "github.com/percybolmer/workflow/failure"
    "github.com/percybolmer/workflow/metric"
    "github.com/percybolmer/workflow/payload"
    "github.com/percybolmer/workflow/properties"
    "github.com/percybolmer/filewatcher"
    "github.com/percybolmer/workflow/processors/processmanager"
    "github.com/percybolmer/workflow/relationships"
    "path/filepath"
    "strings"
)
// ListDirectory is used to list the content of directories on the filesystem
type ListDirectory struct{
    Name     string `json:"name" yaml:"name"`
    running  bool
    cancel   context.CancelFunc
    ingress  relationships.PayloadChannel
    egress   relationships.PayloadChannel
    failures relationships.FailurePipe
    *properties.PropertyMap `json:"properties,omitempty" yaml:"properties,omitempty"`
    *metric.Metrics `json:"metrics,omitempty" yaml:",inline,omitempty"`

    path string
    bufferduration int64
}
// init is used to Register the processor to the processmanager
func init() {
    err := processmanager.RegisterProcessor("ListDirectory", NewListDirectoryInterface)
    if err != nil {
    panic(err)
    }
}
// NewListDirectoryInterface is used to return the Proccssor as interface
// This is to avoid Cyclic imports, we are not allowed to return processors.Processor
// so, let processmanager deal with the type assertion
func NewListDirectoryInterface() interface{} {
    return NewListDirectory()
}

// NewListDirectory is used to initialize and generate a new processor
func NewListDirectory() *ListDirectory {
    proc := &ListDirectory{
        egress: make(relationships.PayloadChannel, 1000),
        PropertyMap: properties.NewPropertyMap(),
        Metrics: metric.NewMetrics(),
    }

    // Add Required Props -- remove_after
    proc.AddRequirement("path")
    return proc
}

// Initialize will make sure all needed Properties and Metrics are generated
func (proc *ListDirectory) Initialize() error {

    // Make sure Properties are there
    ok, _ := proc.ValidateProperties()
    if !ok {
        return properties.ErrRequiredPropertiesNotFulfilled
    }
    // If you need to read data from Properties and add to your Processor struct, this is the place to do it
    pathProp := proc.GetProperty("path")
    proc.path = pathProp.String()

    bufferTimeProp := proc.GetProperty("buffertime")
    if bufferTimeProp != nil {
        time, err := bufferTimeProp.Int()
        if err != nil {
            return err
        }
        proc.bufferduration = int64(time)
    }
    return nil
}

// Start will spawn a goroutine that reads file and Exits either on Context.Done or When processing is finished
func (proc *ListDirectory) Start(ctx context.Context) error {
    if proc.running {
        return failure.ErrAlreadyRunning
    }
    // Uncomment if u need to Processor to require an Ingress relationship
    //if proc.ingress == nil {
    //    return failure.ErrIngressRelationshipNeeded
    //}

    proc.running = true
    // context will be used to spawn a Cancel func
    c, cancel := context.WithCancel(ctx)
    proc.cancel = cancel

    filechannel := make(chan string)

    // Fix filewatcher leaking goroutines
    watcher := filewatcher.NewFileWatcher()
    watcher.WatchDirectory(c, filechannel, proc.path)

    go func(watcher *filewatcher.FileWatcher, filechannel chan string) {
        for {
            select {
                case err := <- watcher.ErrorChan:
                    proc.AddMetric("failures", "the number of failures that has been sent", 1)
                    proc.failures <- failure.Failure{
                        Err:       err,
                        Payload:   nil,
                        Processor: "ListDirectory",
                    }
                case newfile := <-filechannel:
                    proc.AddMetric("files", "the number of files that has been detected", 1)
                    file := filepath.Base(newfile)
                    var filePath string
                    if strings.HasSuffix(proc.path, "/") {
                        filePath = fmt.Sprintf("%s%s", proc.path, file)
                    } else {
                        filePath = fmt.Sprintf("%s/%s", proc.path, file)
                    }
                    payload := payload.BasePayload{
                        Payload: []byte(filePath),
                        Source:  "ListDirectory",
                    }
                    proc.egress <- payload

                case <- c.Done():
                    return
            }
        }
    }(watcher, filechannel)
    return nil
}

// IsRunning will return true or false based on if the processor is currently running
func (proc *ListDirectory) IsRunning() bool {
    return proc.running
}
// GetMetrics will return a bunch of generated metrics, or nil if there isn't any
func (proc *ListDirectory) GetMetrics() []*metric.Metric {
    return proc.GetAllMetrics()
}
// SetFailureChannel will configure the failure channel of the Processor
func (proc *ListDirectory) SetFailureChannel(fp relationships.FailurePipe) {
    proc.failures = fp
}

// Stop will stop the processing
func (proc *ListDirectory) Stop() {
    if !proc.running {
        return
    }
    proc.running = false
    proc.cancel()
}
// SetIngress will change the ingress of the processor, Restart is needed before applied changes
func (proc *ListDirectory) SetIngress(i relationships.PayloadChannel) {
    proc.ingress = i
    return
}
// GetEgress will return an Egress that is used to output the processed items
func (proc *ListDirectory) GetEgress() relationships.PayloadChannel {
    return proc.egress
}