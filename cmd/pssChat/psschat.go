package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/marcusolsson/tui-go"
	flag "github.com/spf13/pflag"
	swarm "github.com/vocdoni/go-dvote/swarm"
)

type Message struct {
	Type int    `json:"type"`
	Nick string `json:"nick"`
	Data string `json:"message"`
}

func main() {
	hostname, _ := os.Hostname()

	kind := flag.String("encryption", "sym", "encryption key schema (raw, sym, asym)")
	key := flag.String("key", "vocdoni", "encryption key (sym or asym)")
	topic := flag.String("topic", "vocdoni_test", "pss topic to subscribe")
	addr := flag.String("address", "", "pss address to send messages")
	nick := flag.String("nick", hostname, "nick name for the pss messages")
	dir := flag.String("datadir", "", "datadir directory for swarm/pss files")
	light := flag.Bool("light", false, "use light mode (less consumption)")
	pingMode := flag.Bool("pingmode", false, "use non interactive ping mode")
	logLevel := flag.String("log", "crit", "log level (info, warn, crit)")
	flag.Parse()

	sn := new(swarm.SimpleSwarm)
	sn.LightNode = *light
	sn.SetDatadir(*dir)

	err := sn.InitPSS()
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	err = sn.SetLog(*logLevel)
	if err != nil {
		fmt.Printf("Cannot set loglevel %v\n", err)
	}

	sn.PssSub(*kind, *key, *topic)
	defer sn.PssTopics[*topic].Unregister()

	log.Info("My PSS pubKey is %s", sn.PssPubKey)

	if *pingMode {
		ping(*topic, *kind, *key, *nick, *addr, sn, *light)
	} else {
		chat(*topic, *kind, *key, *nick, *addr, sn, *light)
	}
}

func ping(topic, enc, key, mynick, addr string, sn *swarm.SimpleSwarm, light bool) {
	go func() {
		var nick string
		var msg string
		var jmsg Message
		for {
			pmsg := <-sn.PssTopics[topic].Delivery
			err := json.Unmarshal(pmsg.Msg, &jmsg)
			if err != nil {
				nick = "raw"
				msg = fmt.Sprintf("%s", pmsg.Msg)
			} else {
				nick = jmsg.Nick
				msg = jmsg.Data
			}
			fmt.Printf("%s <%s>: %s\n", time.Now().Format("3:04PM"), nick, msg)
		}
	}()

	var jmsg Message
	jmsg.Type = 0
	jmsg.Nick = mynick
	for {
		jmsg.Data = "Hello world"
		msg, err := json.Marshal(jmsg)
		if err != nil {
			log.Crit(err.Error())
		}
		err = sn.PssPub(enc, key, topic, fmt.Sprintf("%s", msg), addr)
		if err != nil {
			log.Warn(err.Error())
		}
		time.Sleep(10 * time.Second)
	}
}

func chat(topic, enc, key, mynick, addr string, sn *swarm.SimpleSwarm, light bool) {
	var ui tui.UI
	info := tui.NewHBox()
	info.SetSizePolicy(tui.Expanding, tui.Expanding)
	info.SetBorder(true)

	infoBox := tui.NewScrollArea(info)
	infoBox.SetSizePolicy(tui.Expanding, tui.Expanding)
	infoBox.SetAutoscrollToBottom(false)

	go func() {
		for {
			info.Insert(0, tui.NewHBox(tui.NewLabel("")))
			info.Insert(0,
				tui.NewHBox(
					tui.NewLabel(sn.Hive.String()),
					tui.NewSpacer(),
				))
			time.Sleep(1 * time.Second)
		}
	}()

	sidebar := tui.NewVBox(
		tui.NewLabel(""),
		tui.NewLabel("TOPIC"),
		tui.NewLabel(topic),
		tui.NewLabel(""),
		tui.NewLabel("ENCRYPT"),
		tui.NewLabel(enc),
		tui.NewLabel(""),
		tui.NewLabel("KEY"),
		tui.NewLabel(key),
		tui.NewLabel(""),
		tui.NewLabel("LIGHT"),
		tui.NewLabel(fmt.Sprintf("%t", light)),
		tui.NewLabel(""),

		tui.NewSpacer(),
	)
	sidebar.SetTitle("psschat")
	sidebar.SetBorder(true)

	history := tui.NewVBox()

	go func() {
		var jmsg Message
		var nick string
		var msg string
		for {
			pmsg := <-sn.PssTopics[topic].Delivery
			err := json.Unmarshal(pmsg.Msg, &jmsg)
			if err != nil {
				nick = "raw"
				msg = fmt.Sprintf("%s", pmsg.Msg)
			} else {
				nick = jmsg.Nick
				msg = jmsg.Data
			}
			history.Append(tui.NewHBox(
				tui.NewLabel(time.Now().Format("3:04PM")),
				tui.NewPadder(1, 0, tui.NewLabel(fmt.Sprintf("<%s>", nick))),
				tui.NewLabel(msg),
				tui.NewSpacer(),
			))
			ui.Repaint()
		}
	}()

	historyScroll := tui.NewScrollArea(history)

	historyScroll.SetAutoscrollToBottom(true)

	historyBox := tui.NewVBox(historyScroll)
	historyBox.SetBorder(true)

	input := tui.NewEntry()
	input.SetFocused(true)
	input.SetSizePolicy(tui.Expanding, tui.Maximum)

	inputBox := tui.NewHBox(input)
	inputBox.SetBorder(true)
	inputBox.SetSizePolicy(tui.Expanding, tui.Maximum)

	chat := tui.NewVBox(historyBox, inputBox)
	chat.SetSizePolicy(tui.Expanding, tui.Expanding)

	input.OnSubmit(func(e *tui.Entry) {
		var jmsg Message
		jmsg.Type = 0
		jmsg.Nick = mynick
		jmsg.Data = e.Text()
		msg, err := json.Marshal(jmsg)
		if err != nil {
			log.Crit(err.Error())
		}
		err = sn.PssPub(enc, key, topic, fmt.Sprintf("%s", msg), addr)
		if err != nil {
			log.Warn(err.Error())
		}
		history.Append(tui.NewHBox(
			tui.NewLabel(time.Now().Format("3:04PM")),
			tui.NewPadder(1, 0, tui.NewLabel(fmt.Sprintf("<%s>", mynick))),
			tui.NewLabel(fmt.Sprintf("%s", jmsg.Data)),
			tui.NewSpacer(),
		))
		input.SetText("")
	})

	root := tui.NewHBox(sidebar, chat, infoBox)
	ui, err := tui.New(root)
	if err != nil {
		log.Crit(err.Error())
	}
	quit := false

	ui.SetKeybinding("Esc", func() {
		quit = true
		ui.Quit()
	})
	ui.SetKeybinding("Up", func() { historyScroll.Scroll(0, -1) })
	ui.SetKeybinding("Down", func() { historyScroll.Scroll(0, 1) })
	ui.SetKeybinding("Left", func() { historyScroll.Scroll(-1, 0) })
	ui.SetKeybinding("Right", func() { historyScroll.Scroll(1, 0) })
	ui.SetKeybinding("a", func() { historyScroll.SetAutoscrollToBottom(true) })
	ui.SetKeybinding("t", func() { historyScroll.ScrollToTop() })
	ui.SetKeybinding("b", func() { historyScroll.ScrollToBottom() })

	go func() {
		if err := ui.Run(); err != nil {
			log.Crit(err.Error())
		}
	}()

	for !quit {
		time.Sleep(2 * time.Second)
		ui.Repaint()
	}
}
