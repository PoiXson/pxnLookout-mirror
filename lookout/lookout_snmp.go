package lookout;

import(
	"log"
	"fmt"
	"time"
	"strings"
	"strconv"
	"database/sql"
	gosnmp "github.com/gosnmp/gosnmp"
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
}

type LookoutSnmp interface {
	Run() error
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
	return &LookoutStateSnmp{
		Name:   name,
		Config: config,
		DB:     db,
		Snmp:   &snmp,
		Oids:   oids,
	};
}



func (look LookoutStateSnmp) Close() {
	log.Printf("Close: %s\n", look.Name);
	look.Snmp.Conn.Close();
}



func (look LookoutStateSnmp) Run() error {
	result, err := look.Snmp.Get(look.Oids);
	if err != nil { return err; }
	index := 0;
	for _, node := range look.Config.Snmp.Nodes {
		if strings.HasPrefix(node, "eth-") {
			node = strings.TrimPrefix(node, "eth-");
			if strings.HasPrefix(node, "in-") {
//				idx, err := strconv.Atoi(strings.TrimPrefix(node, "in-"));
//				if err != nil { return err; }
//fmt.Printf("Idx: %d\n", idx);
fmt.Printf(
	"%d Interface: %s %s   Bandwidth: %d\n",
	index,
	result.Variables[index  ].Value,
	result.Variables[index+1].Value,
	result.Variables[index+2].Value,
);
index += 3;
//				oids = append(oids, fmt.Sprintf("1.3.6.1.2.1.31.1.1.1.1.%d",  idx)); // ifName
//				oids = append(oids, fmt.Sprintf("1.3.6.1.2.1.31.1.1.1.18.%d", idx)); // ifAlias
//				oids = append(oids, fmt.Sprintf("1.3.6.1.2.1.31.1.1.1.6.%d",  idx)); // ifHCInOctets
			} else
			if strings.HasPrefix(node, "out-") {
//				idx, err := strconv.Atoi(strings.TrimPrefix(node, "out-"));
//				if err != nil { return err; }
//fmt.Printf("Idx: %d\n", idx);
fmt.Printf(
	"%d Interface: %s %s   Bandwidth: %d\n",
	index,
	result.Variables[index  ].Value,
	result.Variables[index+1].Value,
	result.Variables[index+2].Value,
);
index += 3;
//				oids = append(oids, fmt.Sprintf("1.3.6.1.2.1.31.1.1.1.1.%d",  idx)); // ifName
//				oids = append(oids, fmt.Sprintf("1.3.6.1.2.1.31.1.1.1.18.%d", idx)); // ifAlias
//				oids = append(oids, fmt.Sprintf("1.3.6.1.2.1.31.1.1.1.10.%d", idx)); // ifHCOutOctets
			}
		}
//fmt.Printf("NAME: %s  %v\n", index, node);
	}
	print("\n");
	return nil;
}
