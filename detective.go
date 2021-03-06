package detective

import (
	"github.com/kyleterry/detective/plugins"
	"github.com/op/go-logging"
	stdlog "log"
	"os"
	"sync"
)

var log = logging.MustGetLogger("detective")

// Init is currently used to setup logging. This function might be used for more later.
func Init() {
	logBackend := logging.NewLogBackend(os.Stdout, "", stdlog.LstdFlags)
	logBackend.Color = true
	logging.SetBackend(logBackend)
}

// fanin will merge all the channels dedicated to collecting metrics into one
// channel that CollectAllMetrics() can recieve from.
//
// Returns a channel that all collectors will be redirected to.
func fanin(wg *sync.WaitGroup, chans []<-chan plugins.Result) chan plugins.Result {
	out := make(chan plugins.Result)
	for _, channel := range chans {
		go func(in <-chan plugins.Result) {
			for result := range in {
				out <- result
			}
			wg.Done()
		}(channel)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// CollectAllMetrics will wrap each plugin in a CollectorWrapper, then use fanin
// to merge all the returned channels into one. The out channel is then used to
// build a map of metric results.
//
// Returns a `plugin.Collection` which is a map of Results keyed by the plugin name.
func CollectAllMetrics() plugins.Collection {
	var (
		wg          sync.WaitGroup
		channels    []<-chan plugins.Result
		errchannels []<-chan error
	)
	data := plugins.NewCollection()
	done := make(chan bool)
	wg.Add(RegisteredPlugins.plugins.Len())
	for p := RegisteredPlugins.plugins.Front(); p != nil; p = p.Next() {
		plugin := p.Value.(plugins.DataCollector)
		c, e := plugins.CollectorWrapper(done, plugin)
		channels = append(channels, c)
		errchannels = append(errchannels, e)
	}
	out := fanin(&wg, channels)
	for result := range out {
		data.Items[result.PluginName] = result
	}
	close(done)
	return data
}
