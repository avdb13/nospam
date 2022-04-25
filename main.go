package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	irc "github.com/fluffle/goirc/client"
)

var (
	m        = new(sync.Mutex)
	joined   = make(map[string]time.Time)
	talked   = make(map[string]time.Time)
	warnings = make(map[string]int)
)

func main() {

	nick := "kuroko"
	passwd := "password"
	server := "127.0.0.1"
	port := "6667"
	channel := "#invite"

	infoLog := log.New(os.Stdout, "[INFO] ", log.LUTC)

	cfg := irc.NewConfig(nick)

	cfg.Server = fmt.Sprintf("%s:%s", server, port)
	log.Printf("%s:%s", server, port)
	cfg.NewNick = func(n string) string { return n + "_" }
	cfg.SSL = false
	c := irc.Client(cfg)

	infoLog.Printf("Connecting to %s on %s ...", channel, cfg.Server)

	c.HandleFunc(irc.JOIN,
		func(conn *irc.Conn, line *irc.Line) {
			if line.Nick == nick {
				return
			}

			switch line.Target() {
			case "#invite":
				infoLog.Printf("%s is waiting for an invite!", line.Nick)
				time.Sleep(2 * time.Second)
				conn.Invite(line.Nick, "#general")
			case "#general":
				infoLog.Printf("%s joined!", line.Nick)
				if _, ok := joined[line.Nick]; !ok {
					m.Lock()
					joined[line.Nick] = time.Now()
					warnings[line.Nick] = 2
					m.Unlock()
				}
				time.Sleep(3 * time.Second)
				conn.Mode(line.Nick, "+m")
			}
		})

	c.HandleFunc(irc.PRIVMSG,
		func(conn *irc.Conn, line *irc.Line) {
			if line.Public() {
				m.Lock()
				talked[line.Nick] = line.Time
				m.Unlock()
			}
		})

	c.HandleFunc(irc.CONNECTED,
		func(conn *irc.Conn, line *irc.Line) {
			conn.Oper("admin", passwd)
			conn.Join("#invite")
			conn.Mode(channel, "+N")
			conn.Join("#general")
			conn.Mode("#general", "+im")

			go func() {
				for {
					time.Sleep(1 * time.Second)
					for i := range joined {
						if i == nick {
							return
						}

						if _, ok := talked[i]; !ok {
							talked[i] = joined[i]
						}

						idled := time.Now().Sub(talked[i])
						log.Printf("%s idled for %s\n", i, idled.String())
						if idled.Seconds() > float64(48/warnings[i]) {
							m.Lock()
							warnings[i] *= 2
							m.Unlock()
							conn.Kick("#general", i, "You have been idling for longer than 24h!")
						}
					}
				}
			}()
		})

	quit := make(chan bool)
	c.HandleFunc(irc.DISCONNECTED,
		func(conn *irc.Conn, line *irc.Line) { fmt.Printf("oops!"); quit <- true })

	if err := c.Connect(); err != nil {
		fmt.Printf("Connection error: %s\n", err.Error())
	}

	<-quit
}
