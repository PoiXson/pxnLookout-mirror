package main;

import(
	"os"
	"log"
	"time"
	"sync"
	"syscall"
	"os/signal"
	"io/ioutil"
	"encoding/json"
	Lookout "github.com/PoiXsonGo/pxnLookout/lookout"
);



// defaults
const interval_default = "10s";



func main() {
	print("\n");

	// load monitors.json
	file, err := os.Open("monitors.json");
	if err != nil { log.Panic(err); }
	data, err := ioutil.ReadAll(file);
	file.Close();
	if err != nil { log.Panic(err); }
	var hostconfigs map[string]Lookout.ConfigHost;
	err = json.Unmarshal(data, &hostconfigs);
	if err != nil { log.Panic(err); }
	numhosts := len(hostconfigs);

	// trap ctrl+c
	waitgroup, stopchans := TrapC(numhosts);

	// start monitoring
	log.Printf("Starting %d monitors..", numhosts);
	index := 0;
	for name, cfg := range hostconfigs {
		if cfg.Enable {
			mon := Lookout.New(
				interval_default,
				name, cfg,
				waitgroup,
				stopchans[index],
			);
			go mon.Run();
		}
		index++;
	}
	time.Sleep(time.Second);
	print("\n");
	waitgroup.Wait();
	print("\n");
	os.Exit(0);
}



func TrapC(size int) (*sync.WaitGroup, []chan bool) {
	var waitgroup sync.WaitGroup;
	stopchans := make([]chan bool, size);
	for i := range stopchans {
		stopchans[i] = make(chan bool, 1);
	}
	sig := make(chan os.Signal, 1);
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM);
	go func() {
		sigcount := 0;
		for {
			<-sig;
			switch sigcount {
			case 0:
				print("\rStopping..\n");
				for i := range stopchans {
					stopchans[i] <- true;
				}
				go func() {
					waitgroup.Wait();
					print("Exit(0)");
					os.Exit(0);
				}();
				break;
			case 1:  print("\rTerminate?\n"); break;
			default: print("\rTerminate!\n"); os.Exit(0);
			}
			sigcount++;
		}
	}();
	return &waitgroup, stopchans;
}
