package lookout;

import(
	"os"
	"log"
	"fmt"
	"time"
	"sync"
	"strings"
	"strconv"
	"io/ioutil"
	"database/sql"
	_ "github.com/tursodatabase/go-libsql"
);



const microinterval = time.Millisecond * 800;
const driver = "libsql";
const dsn = "file:%s.db?_pragma=journal_mode=WAL&_pragma=synchronous=NORMAL";
const dbPath ="db/";



type ConfigHost struct {
	Enable   bool   `json:"Enable"`
	Host     string `json:"Host"`
	Interval string `json:"Interval,omitempty"`
	Snmp *ConfigProtocolSnmp `json:"SNMP,omitempty"`
}

type Protocol interface {
	Connect()
	Update()
}



type LookoutState struct {
	Name      string
	Config    ConfigHost
	Interval  int
	DB        *sql.DB
	WaitGroup *sync.WaitGroup
	StopChan  <-chan bool
	Snmp      *LookoutStateSnmp
}

type Lookout interface {
	Run()
}



func New(interval string, name string, config ConfigHost,
waitgroup *sync.WaitGroup, stopchan chan bool) LookoutState {
	if config.Interval != "" {
		interval = config.Interval;
	}
	intervalSecs, err := ParseTimeToSeconds(interval);
	if err != nil { log.Panic(err); }
	// connect db
	db, err := GetDB(name);
	if err != nil { log.Panic(err); }
	// connect snmp
	var snmp *LookoutStateSnmp;
	if config.Snmp != nil {
		snmp = NewSnmp(name, config, db);
	}
	// monitor instance
	look := LookoutState{
		Name:      name,
		Config:    config,
		Interval:  intervalSecs,
		DB:        db,
		WaitGroup: waitgroup,
		StopChan:  stopchan,
		Snmp:      snmp,
	};
	return look;
}



func (look LookoutState) Run() {
	look.WaitGroup.Add(1);
	defer func() {
		if look.Config.Snmp != nil {
			look.Snmp.Close();
		}
		if err := look.DB.Close(); err != nil {
			log.Printf("Error closing database: %v", err);
		}
		look.WaitGroup.Done();
	}();
	// monitor loop
	fmt.Printf("Monitoring Server: [%s] %s\n", look.Name, look.Config.Host);
	for loops:=1; ; loops++ {
		now := time.Now();
		interval := time.Duration(look.Interval) * time.Second;
		next := now.Truncate(interval).Add(interval);
		sleep := next.Sub(now);
		// first loop minimum 2 seconds
		if loops == 1 && sleep <= 2 * time.Second {
			sleep += interval;
		}
		// sleep loop
		for sleep > 0 {
			select {
			case stopping := <-look.StopChan:
				if stopping {
					fmt.Printf(
						"Stopping monitor: [%s] %s\n",
						look.Name, look.Config.Host,
					);
					return;
				}
			default:
			}
			// sleep a short moment
			if sleep > microinterval {
				time.Sleep(microinterval);
				sleep -= microinterval;
			// final sleep
			} else {
				time.Sleep(sleep);
				sleep = 0;
			}
		}
//TODO: need to round this to nearest time unit
		var tim  int64 = now.Unix();
		// update snmp
		if look.Snmp != nil {
			err := look.Snmp.Run(tim);
			if err != nil {
				log.Printf("Error: <%s>\n  %v\n\n", look.Name, err);
			}
		}
	}
}



func GetDB(key string) (*sql.DB, error) {
	// create db dir
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		if err := os.Mkdir(dbPath, 0755); err != nil { return nil, err; }
	}
	// connect db
	db, err := sql.Open(driver, fmt.Sprintf(dsn, dbPath+key));
	if err != nil { return nil, err; }
	if err := db.Ping(); err != nil {
		return nil, err;
	}
	// load and run schema
	schema, err := ioutil.ReadFile("schema.sql");
	if err != nil { return nil, err; }
	if _, err := db.Exec(string(schema)); err != nil { return nil, err; }
	return db, nil;
}



func ParseTimeToSeconds(str string) (int, error) {
	var result int;
	var err error;
	if strings.HasSuffix(str, "s") { result, err = strconv.Atoi(str[:len(str)-1]);                             } else // seconds
	if strings.HasSuffix(str, "m") { result, err = strconv.Atoi(str[:len(str)-1]); result *= 60;               } else // minutes
	if strings.HasSuffix(str, "h") { result, err = strconv.Atoi(str[:len(str)-1]); result *= 60 * 60;          } else // hours
	if strings.HasSuffix(str, "d") { result, err = strconv.Atoi(str[:len(str)-1]); result *= 60 * 60 * 24;     } else // days
	if strings.HasSuffix(str, "w") { result, err = strconv.Atoi(str[:len(str)-1]); result *= 60 * 60 * 24 * 7; } else // weeks
		{ result, err = strconv.Atoi(str); } // default
	return result, err;
}



func FormatBandwidthPerSecond(bw int64) string {
	bw = bw / 10;
	if bw > 1000000000000 { return fmt.Sprintf("%dTb/s", bw/1000000000000); }
	if bw >    1000000000 { return fmt.Sprintf("%dGb/s",    bw/1000000000); }
	if bw >       1000000 { return fmt.Sprintf("%dMb/s",       bw/1000000); }
	if bw >          1000 { return fmt.Sprintf("%dKb/s",          bw/1000); }
	return fmt.Sprintf("%dB/s", bw);
}
func FormatBandwidth(bw uint64) string {
	if bw > 1000000000000 { return fmt.Sprintf("%dT", bw/1000000000000); }
	if bw >    1000000000 { return fmt.Sprintf("%dG",    bw/1000000000); }
	if bw >       1000000 { return fmt.Sprintf("%dM",       bw/1000000); }
	if bw >          1000 { return fmt.Sprintf("%dK",          bw/1000); }
	return fmt.Sprintf("%dB", bw);
}
