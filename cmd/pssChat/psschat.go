package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"time"

	"github.com/marcusolsson/tui-go"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/log"
	swarm "gitlab.com/vocdoni/go-dvote/swarm"
)

func newConfig() (config.PssCfg, error) {
	var globalCfg config.PssCfg
	//setup flags
	usr, err := user.Current()
	if err != nil {
		return globalCfg, err
	}
	defaultDirPath := usr.HomeDir + "/.dvote/psschat"
	path := flag.String("cfgpath", defaultDirPath+"/config.yaml", "cfgpath. Specify filepath for psschat config")

	flag.String("encryption", "sym", "encryption key schema (raw, sym, asym)")
	flag.String("key", "vocdoni", "encryption key (sym or asym)")
	flag.String("topic", "vocdoni_test", "pss topic to subscribe")
	flag.String("address", "", "pss address to send messages")
	flag.String("bootnode", "", "pss custom peer bootnode")
	flag.String("nick", "", "nick name for the pss messages")
	flag.String("datadir", "", "datadir directory for swarm/pss files")
	flag.Bool("light", false, "use light mode (less consumption)")
	flag.Bool("pingmode", false, "use non interactive ping mode")
	flag.String("loglevel", "warn", "Log level. Valid values are: debug, info, warn, error, dpanic, panic, fatal.")
	flag.Parse()

	viper := viper.New()
	viper.SetDefault("encryption", "sym")
	viper.SetDefault("key", "vocdoni")
	viper.SetDefault("topic", "vocdoni_test")
	viper.SetDefault("address", "")
	viper.SetDefault("bootnode", "")
	viper.SetDefault("nick", "")
	viper.SetDefault("datadir", "")
	viper.SetDefault("light", false)
	viper.SetDefault("pingmode", false)
	viper.SetDefault("loglevel", "warn")

	viper.SetConfigType("yaml")
	if *path == defaultDirPath+"/config.yaml" { //if path left default, write new cfg file if empty or if file doesn't exist.
		if err = viper.SafeWriteConfigAs(*path); err != nil {
			if os.IsNotExist(err) {
				err = os.MkdirAll(defaultDirPath, os.ModePerm)
				if err != nil {
					return globalCfg, err
				}
				err = viper.WriteConfigAs(*path)
				if err != nil {
					return globalCfg, err
				}
			}
		}
	}

	viper.BindPFlag("encryption", flag.Lookup("encryption"))
	viper.BindPFlag("key", flag.Lookup("key"))
	viper.BindPFlag("topic", flag.Lookup("topic"))
	viper.BindPFlag("address", flag.Lookup("address"))
	viper.BindPFlag("bootnode", flag.Lookup("bootnode"))
	viper.BindPFlag("nick", flag.Lookup("nick"))
	viper.BindPFlag("datadir", flag.Lookup("datadir"))
	viper.BindPFlag("light", flag.Lookup("light"))
	viper.BindPFlag("pingmode", flag.Lookup("pingmode"))
	viper.BindPFlag("logLevel", flag.Lookup("loglevel"))

	viper.SetConfigFile(*path)
	err = viper.ReadInConfig()
	if err != nil {
		return globalCfg, err
	}

	err = viper.Unmarshal(&globalCfg)
	return globalCfg, err
}

//Message holds a pss chat message
type Message struct {
	Type int    `json:"type"`
	Nick string `json:"nick"`
	Data string `json:"message"`
}

func main() {
	//setup config
	globalCfg, err := newConfig()
	//setup logger
	log.InitLogger(globalCfg.LogLevel, "stdout")
	if err != nil {
		log.Fatalf("Could not load config: %v", err)
	}
	if globalCfg.Nick == "" {
		globalCfg.Nick, _ = os.Hostname()
	}
	sn := new(swarm.SimpleSwarm)
	sn.LightNode = globalCfg.Light
	sn.SetDatadir(globalCfg.Datadir)

	bootNodes := swarm.SwarmBootnodes
	if globalCfg.Bootnode != "" {
		log.Infof("Using custom bootNode %s", globalCfg.Bootnode)
		bootNodes = []string{globalCfg.Bootnode}
	}

	err = sn.InitPSS(bootNodes)
	if err != nil {
		log.Errorf("%v\n", err)
		return
	}

	sn.PssSub(globalCfg.Encryption, globalCfg.Key, globalCfg.Topic)
	defer sn.PssTopics[globalCfg.Topic].Unregister()

	log.Infof("My PSS pubKey is %s", sn.PssPubKey)

	if globalCfg.PingMode {
		ping(globalCfg, sn)
	} else {
		chat(globalCfg, sn)
	}
}

func ping(globalCfg config.PssCfg, sn *swarm.SimpleSwarm) {
	go func() {
		var nick string
		var msg string
		var jmsg Message
		for {
			pmsg := <-sn.PssTopics[globalCfg.Topic].Delivery
			err := json.Unmarshal(pmsg.Msg, &jmsg)
			if err != nil {
				nick = "raw"
				msg = fmt.Sprintf("%s", pmsg.Msg)
			} else {
				nick = jmsg.Nick
				msg = jmsg.Data
			}
			log.Infof("Message info: Time: %v, Nick: %v, Message: %v", time.Now().Format("3:04PM"), nick, msg)
		}
	}()

	var jmsg Message
	jmsg.Type = 0
	jmsg.Nick = globalCfg.Nick
	for {
		jmsg.Data = "Hello world"
		msg, err := json.Marshal(jmsg)
		if err != nil {
			log.Fatal(err)
		}
		err = sn.PssPub(globalCfg.Encryption, globalCfg.Key, globalCfg.Topic, fmt.Sprintf("%s", msg), globalCfg.Address)
		if err != nil {
			log.Warn(err)
		}
		time.Sleep(10 * time.Second)
	}
}

func chat(globalCfg config.PssCfg, sn *swarm.SimpleSwarm) {
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
		tui.NewLabel(globalCfg.Topic),
		tui.NewLabel(""),
		tui.NewLabel("ENCRYPT"),
		tui.NewLabel(globalCfg.Encryption),
		tui.NewLabel(""),
		tui.NewLabel("KEY"),
		tui.NewLabel(globalCfg.Key),
		tui.NewLabel(""),
		tui.NewLabel("LIGHT"),
		tui.NewLabel(fmt.Sprintf("%t", globalCfg.Light)),
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
			pmsg := <-sn.PssTopics[globalCfg.Topic].Delivery
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
		jmsg.Nick = globalCfg.Nick
		jmsg.Data = e.Text()
		msg, err := json.Marshal(jmsg)
		if err != nil {
			log.Fatal(err)
		}
		err = sn.PssPub(globalCfg.Encryption, globalCfg.Key, globalCfg.Topic, fmt.Sprintf("%s", msg), globalCfg.Address)
		if err != nil {
			log.Warn(err)
		}
		history.Append(tui.NewHBox(
			tui.NewLabel(time.Now().Format("3:04PM")),
			tui.NewPadder(1, 0, tui.NewLabel(fmt.Sprintf("<%s>", globalCfg.Nick))),
			tui.NewLabel(fmt.Sprintf("%s", jmsg.Data)),
			tui.NewSpacer(),
		))
		input.SetText("")
	})

	root := tui.NewHBox(sidebar, chat, infoBox)
	ui, err := tui.New(root)
	if err != nil {
		log.Fatal(err)
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
			log.Fatal(err)
		}
	}()

	for !quit {
		time.Sleep(2 * time.Second)
		ui.Repaint()
	}
}
