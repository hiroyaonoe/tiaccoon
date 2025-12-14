package cni

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
)

type Handler struct {
}

func (h *Handler) Start(tiaccoonVersion string) {
	skel.PluginMainFuncs(
		skel.CNIFuncs{
			Add:    h.Add,
			Del:    h.Del,
			Check:  h.Check,
			GC:     h.GC,
			Status: h.Status,
		},
		version.PluginSupports("0.4.0", "1.0.0", "1.1.0"),
		fmt.Sprintf("CNI plugin tiaccoon %s", tiaccoonVersion),
	)
}

func (h *Handler) Add(args *skel.CmdArgs) error {
	err := logCmdArgs("ADD", args)
	if err != nil {
		return err
	}
	return printPrevResult(args)
}

func (h *Handler) Del(args *skel.CmdArgs) error {
	err := logCmdArgs("DEL", args)
	if err != nil {
		return err
	}
	return printPrevResult(args)
}

func (h *Handler) Check(args *skel.CmdArgs) error {
	err := logCmdArgs("CHECK", args)
	if err != nil {
		return err
	}
	return printPrevResult(args)
}

func (h *Handler) GC(args *skel.CmdArgs) error {
	err := logCmdArgs("GC", args)
	if err != nil {
		return err
	}
	return printPrevResult(args)
}

func (h *Handler) Status(args *skel.CmdArgs) error {
	err := logCmdArgs("STATUS", args)
	if err != nil {
		return err
	}
	return printPrevResult(args)
}

func logCmdArgs(cmd string, args *skel.CmdArgs) error {
	str := fmt.Sprintf("Command: %s\n", cmd)
	str += fmt.Sprintf("	ContainerID: %s\n", args.ContainerID)
	str += fmt.Sprintf("	Netns: %s\n", args.Netns)
	str += fmt.Sprintf("	IfName: %s\n", args.IfName)
	str += fmt.Sprintf("	Args: %s\n", args.Args)
	str += fmt.Sprintf("	Path: %s\n", args.Path)
	str += fmt.Sprintf("	NetnsOverride: %s\n", args.NetnsOverride)
	str += fmt.Sprintf("	StdinData: %s\n", args.StdinData)

	err := os.MkdirAll("/var/run/tiaccoon", 0755)
	if err != nil {
		return err
	}
	f, err := os.OpenFile("/var/run/tiaccoon/tiaccoon-cni.log", os.O_APPEND|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, str)
	return err
}

func parseNetConf(data []byte) (*types.NetConf, error) {
	conf := &types.NetConf{}
	err := json.Unmarshal(data, conf)
	if err != nil {
		return nil, err
	}
	return conf, nil
}

func printPrevResult(args *skel.CmdArgs) error {
	conf, err := parseNetConf(args.StdinData)
	if err != nil {
		return err
	}
	if conf.RawPrevResult == nil {
		return fmt.Errorf("Required prevResult missing")
	}

	if err := version.ParsePrevResult(conf); err != nil {
		return err
	}
	result, err := current.NewResultFromResult(conf.PrevResult)
	if err != nil {
		return err
	}

	return types.PrintResult(result, conf.CNIVersion)
}
