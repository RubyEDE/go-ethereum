// Copyright 2024 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/ethereum/go-ethereum/beacon/blsync"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/internal/debug"
	"github.com/ethereum/go-ethereum/internal/flags"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/urfave/cli/v2"
)

func main() {
	app := flags.NewApp("beacon light syncer tool")
	app.Flags = slices.Concat([]cli.Flag{
		utils.BeaconApiFlag,
		utils.BeaconApiHeaderFlag,
		utils.BeaconThresholdFlag,
		utils.BeaconNoFilterFlag,
		utils.BeaconConfigFlag,
		utils.BeaconGenesisRootFlag,
		utils.BeaconGenesisTimeFlag,
		utils.BeaconCheckpointFlag,
		utils.BeaconCheckpointFileFlag,
		//TODO datadir for optional permanent database
		utils.MainnetFlag,
		utils.SepoliaFlag,
		utils.HoleskyFlag,
		utils.HoodiFlag,
		utils.BlsyncApiFlag,
		utils.BlsyncJWTSecretFlag,
		utils.DataDirFlag,
	},
		debug.Flags,
	)
	app.Before = func(ctx *cli.Context) error {
		flags.MigrateGlobalFlags(ctx)
		return debug.Setup(ctx)
	}
	app.After = func(ctx *cli.Context) error {
		debug.Exit()
		return nil
	}
	app.Action = sync

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func sync(ctx *cli.Context) error {
	// set up blsync
	client := blsync.NewClient(utils.MakeBeaconLightConfig(ctx))
	client.SetEngineRPC(makeRPCClient(ctx))
	client.Start()

	// run until stopped
	<-ctx.Done()
	client.Stop()
	return nil
}

func makeRPCClient(ctx *cli.Context) *rpc.Client {
	if !ctx.IsSet(utils.BlsyncApiFlag.Name) {
		log.Warn("No engine API target specified, performing a dry run")
		return nil
	}

	engineApiUrl := ctx.String(utils.BlsyncApiFlag.Name)
	var jwtFileName string

	if ctx.IsSet(utils.BlsyncJWTSecretFlag.Name) {
		jwtFileName = ctx.String(utils.BlsyncJWTSecretFlag.Name)
	} else if ctx.IsSet(utils.DataDirFlag.Name) {
		// Use default JWT secret path in the specified datadir
		datadir := ctx.String(utils.DataDirFlag.Name)
		jwtFileName = filepath.Join(datadir, "jwtsecret")
	} else {
		utils.Fatalf("JWT secret parameter missing and no datadir specified. Use --blsync.jwtsecret or --datadir")
	}

	var jwtSecret [32]byte
	if jwt, err := node.ObtainJWTSecret(jwtFileName); err == nil {
		copy(jwtSecret[:], jwt)
	} else {
		utils.Fatalf("Error loading or generating JWT secret: %v", err)
	}
	auth := node.NewJWTAuth(jwtSecret)
	cl, err := rpc.DialOptions(context.Background(), engineApiUrl, rpc.WithHTTPAuth(auth))
	if err != nil {
		utils.Fatalf("Could not create RPC client: %v", err)
	}
	return cl
}
