package control

import (
	"bufio"
	"fmt"
	"os"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	composeConfig "github.com/docker/libcompose/config"
	"github.com/docker/libcompose/project/options"
	"github.com/rancher/os/compose"
	"github.com/rancher/os/config"
	"github.com/rancher/os/util/network"
)

func consoleSubcommands() []cli.Command {
	return []cli.Command{
		{
			Name:   "switch",
			Usage:  "switch console without a reboot",
			Action: consoleSwitch,
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "force, f",
					Usage: "do not prompt for input",
				},
			},
		},
		{
			Name:   "enable",
			Usage:  "set console to be switched on next reboot",
			Action: consoleEnable,
		},
		{
			Name:   "list",
			Usage:  "list available consoles",
			Action: consoleList,
		},
	}
}

func consoleSwitch(c *cli.Context) error {
	if len(c.Args()) != 1 {
		log.Fatal("Must specify exactly one console to switch to")
	}
	newConsole := c.Args()[0]

	cfg := config.LoadConfig()
	if newConsole == cfg.Rancher.Console {
		log.Warnf("Console is already set to %s", newConsole)
	}

	if !c.Bool("force") {
		in := bufio.NewReader(os.Stdin)
		fmt.Println("Switching consoles will destroy the current console container and restart Docker.")
		fmt.Println("Note: You will also be logged out.")
		if !yes(in, "Continue") {
			return nil
		}
	}

	if newConsole != "default" {
		if err := compose.StageServices(cfg, newConsole); err != nil {
			return err
		}
	}

	service, err := compose.CreateService(nil, "switch-console", &composeConfig.ServiceConfigV1{
		LogDriver:  "json-file",
		Privileged: true,
		Net:        "host",
		Pid:        "host",
		Image:      fmt.Sprintf("rancher/os-base:%s", config.VERSION),
		Labels: map[string]string{
			config.SCOPE: config.SYSTEM,
		},
		Command:     []string{"/usr/bin/switch-console", newConsole},
		VolumesFrom: []string{"all-volumes"},
	})
	if err != nil {
		return err
	}

	if err = service.Delete(context.Background(), options.Delete{}); err != nil {
		return err
	}
	if err = service.Up(context.Background(), options.Up{}); err != nil {
		return err
	}
	return service.Log(context.Background(), true)
}

func consoleEnable(c *cli.Context) error {
	if len(c.Args()) != 1 {
		log.Fatal("Must specify exactly one console to enable")
	}
	newConsole := c.Args()[0]

	cfg := config.LoadConfig()

	if newConsole != "default" {
		if err := compose.StageServices(cfg, newConsole); err != nil {
			return err
		}
	}

	if err := config.Set("rancher.console", newConsole); err != nil {
		log.Errorf("Failed to update 'rancher.console': %v", err)
	}

	return nil
}

func consoleList(c *cli.Context) error {
	cfg := config.LoadConfig()

	consoles, err := network.GetConsoles(cfg.Rancher.Repositories.ToArray())
	if err != nil {
		return err
	}

	fmt.Println("default")
	for _, console := range consoles {
		fmt.Println(console)
	}

	return nil
}
