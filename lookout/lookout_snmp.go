package lookout;

import(
	"log"
	"fmt"
	"time"
	"errors"
	"strings"
	"strconv"
	"context"
	"runtime"
	"database/sql"
	_      "github.com/tursodatabase/go-libsql"
	gosnmp "github.com/gosnmp/gosnmp"
	"github.com/PoiXsonGo/pxnLookout/lookout/metrics"
);



type ConfigProtocolSnmp struct {
	Community string `json:"Community"`
	Nodes   []string `json:"Nodes"`
}



type LookoutStateSnmp struct {
	Name   string
	Config ConfigHost
	DB     *sql.DB
	Snmp   *gosnmp.GoSNMP
	Oids   []string
	Eths   map[string]uint64
}

type LookoutSnmp interface {
	Run(tim int64) error
	RunQuery_UpdateRecord(ctx *context.Context, qry *metrics.Queries,
			id int, time int64, bw int64) error
	RunError(e error) error
	Close()
}



func NewSnmp(name string, config ConfigHost, db *sql.DB) *LookoutStateSnmp {
//TODO: more configurable parameters
	snmpVersion := gosnmp.Version2c;
	snmp := gosnmp.GoSNMP{
		Target:    config.Host,
		Port:      161,
		Community: config.Snmp.Community,
		Version:   snmpVersion,
		Timeout:   time.Duration(5 * time.Second),
	};
	// connect snmp
	if err := snmp.Connect(); err != nil { log.Panic(err); }
	var oids []string;
	for _, node := range config.Snmp.Nodes {
		if strings.HasPrefix(node, "eth-") {
			node = strings.TrimPrefix(node, "eth-");
			if strings.HasPrefix(node, "in-") {
				idx, err := strconv.Atoi(strings.TrimPrefix(node, "in-"));
				if err != nil { log.Panic(err); }
				oids = append(oids, fmt.Sprintf("1.3.6.1.2.1.31.1.1.1.1.%d",  idx)); // ifName
				oids = append(oids, fmt.Sprintf("1.3.6.1.2.1.31.1.1.1.18.%d", idx)); // ifAlias
				oids = append(oids, fmt.Sprintf("1.3.6.1.2.1.31.1.1.1.6.%d",  idx)); // ifHCInOctets
			} else
			if strings.HasPrefix(node, "out-") {
				idx, err := strconv.Atoi(strings.TrimPrefix(node, "out-"));
				if err != nil { log.Panic(err); }
				oids = append(oids, fmt.Sprintf("1.3.6.1.2.1.31.1.1.1.1.%d",  idx)); // ifName
				oids = append(oids, fmt.Sprintf("1.3.6.1.2.1.31.1.1.1.18.%d", idx)); // ifAlias
				oids = append(oids, fmt.Sprintf("1.3.6.1.2.1.31.1.1.1.10.%d", idx)); // ifHCOutOctets
			}
		}
	}
	ethernets := make(map[string]uint64);
	return &LookoutStateSnmp{
		Name:   name,
		Config: config,
		DB:     db,
		Snmp:   &snmp,
		Oids:   oids,
		Eths:   ethernets,
	};
}



func (look LookoutStateSnmp) Close() {
	log.Printf("Close: %s\n", look.Name);
	look.Snmp.Conn.Close();
}



func (look LookoutStateSnmp) Run(tim int64) error {
	result, err := look.Snmp.Get(look.Oids);
	if err != nil { return look.RunError(err); }
	index := 0;
//TODO: don't use idx for this; need to use a new db table for server/host/protocol/community+node
	for idx, node := range look.Config.Snmp.Nodes {
		nod := node;
		if strings.HasPrefix(nod, "eth-") {
			nod = strings.TrimPrefix(nod, "eth-");
			if strings.HasPrefix(nod, "in-") {
				eth := result.Variables[index+1].Value;
				if eth == "" {
					eth = result.Variables[index].Value;
				}
				value := result.Variables[index+2].Value.(uint64);
				last := look.Eths[node];
				if last > 0 {
					if last > value { return errors.New("Invalid value > last"); }
					var bw int64 = int64(value) - int64(last);
					fmt.Printf("%d) Interface: %s in %s\n", index, eth, FormatBandwidthPerSecond(bw));
					ctx := context.Background();
					qry := metrics.New(look.DB);
					// update record
					err := look.RunQuery_UpdateRecord(&ctx, qry, idx, tim, bw);
					if err != nil { return look.RunError(err); }
				}
				// last bandwidth value
				look.Eths[node] = value;
				index += 3;
//				idx, err := strconv.Atoi(strings.TrimPrefix(nod, "in-"));
//				if err != nil { return err; }
			} else
			if strings.HasPrefix(nod, "out-") {
				eth := result.Variables[index+1].Value;
				if eth == "" {
					eth = result.Variables[index].Value;
				}
				value := result.Variables[index+2].Value.(uint64);
				last := look.Eths[node];
				if last > 0 {
					if last > value { return errors.New("Invalid value > last"); }
					var bw int64 = int64(value) - int64(last);
					fmt.Printf("%d) Interface: %s out %s\n", index, eth, FormatBandwidthPerSecond(bw));
					ctx := context.Background();
					qry := metrics.New(look.DB);
					// update record
					err := look.RunQuery_UpdateRecord(&ctx, qry, idx, tim, bw);
					if err != nil { return look.RunError(err); }
				}
				look.Eths[node] = value;
				index += 3;
			}
		}
	}
	print("\n");
	return nil;
}

func (look LookoutStateSnmp) RunQuery_UpdateRecord(ctx *context.Context, qry *metrics.Queries,
		id int, time int64, bw int64) error {
	paramsCreate := metrics.CreateRecordParams{
		Node: int64(id),
		Time: int64(time),
	};
	paramsUpdate := metrics.UpdateRecordParams{
		Value: int64(bw),
		Node:  int64(id),
		Time:  int64(time),
	};
	if _, err := qry.UpdateRecord(*ctx, paramsUpdate); err == nil { return nil;                } // update record
	if    err := qry.CreateRecord(*ctx, paramsCreate); err != nil { return look.RunError(err); } // create record if needed
	if _, err := qry.UpdateRecord(*ctx, paramsUpdate); err != nil { return look.RunError(err); } // update record
	return nil;
}

func (look LookoutStateSnmp) RunError(e error) error {
	if _, file, line, ok := runtime.Caller(1); ok {
		return fmt.Errorf("Look failed: %d:%s\n  %w", line, file, e);
	}
	return e;
}
