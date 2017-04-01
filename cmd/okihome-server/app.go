// Copyright 2016 Simon HEGE. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	_ "github.com/lib/pq"
	"github.com/oki-apps/okihome"
	"github.com/oki-apps/okihome/api"
	"github.com/oki-apps/okihome/logInteractor/console"
	"github.com/oki-apps/okihome/providers/gmail"
	"github.com/oki-apps/okihome/providers/outlook"
	"github.com/oki-apps/okihome/repository/postgresql"
	okihomeServer "github.com/oki-apps/okihome/server"
	"github.com/oki-apps/okihome/userInteractor/contextUser"
	"github.com/oki-apps/server"
)

type config struct {
	Server     server.Config
	Postgresql *postgresql.Config
	Gmail      *gmail.Config
	Outlook    *outlook.Config
}

func readConfig() config {
	var cfg config

	path := "okihome.json"
	if len(os.Args) >= 2 {
		path = os.Args[1]
	}

	b, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = json.Unmarshal(b, &cfg)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Configuration read from ", path)

	return cfg
}

func main() {

	cfg := readConfig()

	//Instantiate all components

	//DatabaseConnector
	var repo api.Repository
	if cfg.Postgresql != nil {
		var err error
		repo, err = postgresql.New(*cfg.Postgresql)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		fmt.Println("Missing PostgreSQL configuration")
		os.Exit(1)
	}

	//Log
	logInteractor := console.New()

	//User
	userInteractor := contextUser.New()

	//Services provider
	var providers []api.Provider
	if cfg.Gmail != nil {
		gmailProvider := gmail.New(*cfg.Gmail, repo)
		providers = append(providers, gmailProvider)
	}
	if cfg.Outlook != nil {
		outlookProvider := outlook.New(*cfg.Outlook, repo)
		providers = append(providers, outlookProvider)
	}

	app := okihome.NewApp(repo, userInteractor, logInteractor, providers)

	//Server
	s, err := okihomeServer.New(app, cfg.Server)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	//Start web app
	if err := s.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
