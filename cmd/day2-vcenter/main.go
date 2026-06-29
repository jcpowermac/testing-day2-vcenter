package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	"github.com/jcallen/testing-day2-vcenter/pkg/lab"
	"github.com/jcallen/testing-day2-vcenter/pkg/labconfig"
)

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		usage()
		return 2
	}

	switch os.Args[1] {
	case "apply":
		return runApply(os.Args[2:])
	case "restore":
		return runRestore(os.Args[2:])
	case "verify":
		return runVerify(os.Args[2:])
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		return 2
	}
}

func runApply(args []string) int {
	fs := flag.NewFlagSet("apply", flag.ExitOnError)
	configPath := fs.String("config", labconfig.DefaultConfigPath, "path to lab config YAML")
	dryRun := fs.Bool("dry-run", false, "validate changes without persisting")
	_ = fs.Parse(args)

	cfg, err := labconfig.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	clients, err := framework.NewClients()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	result, err := lab.Apply(ctx, clients, cfg, lab.ApplyOptions{DryRun: *dryRun})
	if err != nil {
		fmt.Fprintf(os.Stderr, "apply failed: %v\n", err)
		return 1
	}

	fmt.Printf("added vCenter %q", result.AddedVCenter)
	if result.AddedFailureDomain != "" {
		fmt.Printf(" with failure domain %q", result.AddedFailureDomain)
	}
	if *dryRun {
		fmt.Printf(" (dry-run)\n")
	} else {
		fmt.Printf("\nbackup saved to %s\n", result.StateDir)
	}
	return 0
}

func runRestore(args []string) int {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	configPath := fs.String("config", labconfig.DefaultConfigPath, "path to lab config YAML")
	stateDir := fs.String("state-dir", "", "override state backup directory")
	_ = fs.Parse(args)

	cfg, err := labconfig.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	dir := cfg.StateDir
	if *stateDir != "" {
		dir = *stateDir
	}

	clients, err := framework.NewClients()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	if err := lab.Restore(ctx, clients, dir); err != nil {
		fmt.Fprintf(os.Stderr, "restore failed: %v\n", err)
		return 1
	}
	fmt.Printf("restored cluster state from %s\n", dir)
	return 0
}

func runVerify(args []string) int {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	configPath := fs.String("config", labconfig.DefaultConfigPath, "path to lab config YAML")
	_ = fs.Parse(args)

	cfg, err := labconfig.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	clients, err := framework.NewClients()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := lab.Verify(ctx, clients, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "verify failed: %v\n", err)
		return 1
	}
	fmt.Printf("vCenter %q is present and operators are healthy\n", cfg.SecondVCenter.Server)
	return 0
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: day2-vcenter <command> [flags]

Commands:
  apply    Add second vCenter from config/lab.yaml (backs up cluster first)
  restore  Restore Infrastructure, cloud-provider-config, and credential secrets
  verify   Check operators and managed cloud config include the vCenter

Examples:
  cp config/lab.yaml.example config/lab.yaml   # edit with your vCenter
  go run ./cmd/day2-vcenter apply -config config/lab.yaml
  go run ./cmd/day2-vcenter verify -config config/lab.yaml
  go run ./cmd/day2-vcenter restore -config config/lab.yaml

Environment:
  KUBECONFIG   Path to cluster admin kubeconfig (required)

`)
}
