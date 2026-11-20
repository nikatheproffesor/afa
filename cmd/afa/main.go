package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"time"

	"afa/internal/buildinfo"
	"afa/internal/platform"
	"afa/internal/portal"
	"afa/internal/service"
	"afa/internal/speed"
)

const help = `AFA · Access for All
Wi-Fi girişi, servis erişimi ve bağlantı kalitesi tanısı.

  afa portal [--open] [--json]
  afa check [--json] HTTPS_URL
  afa check [--json] --profile discord|roblox|steam|epic
  afa speed [--mib 1–8] [--samples 1–5] [--json]
  afa quality [--samples 2–8] [--json]
  afa version

Bir komutun seçenekleri için komut adından sonra --help kullanın.
`

type operations struct {
	portal  func(context.Context) portal.Result
	open    func(context.Context) error
	service func(context.Context, string) (service.Result, error)
	profile func(context.Context, string) (service.ProfileResult, error)
	speed   func(context.Context, int) (speed.Result, error)
	quality func(context.Context, int) (speed.QualityResult, error)
}

var systemOperations = operations{
	portal: portal.Check, open: platform.OpenPortal, service: service.Check,
	profile: service.CheckProfile, speed: speed.Check, quality: speed.Quality,
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	cancel()
	os.Exit(code)
}

func run(ctx context.Context, args []string, out, errOut io.Writer) int {
	return runWith(ctx, args, out, errOut, systemOperations)
}

func runWith(ctx context.Context, args []string, out, errOut io.Writer, ops operations) int {
	if len(args) == 0 {
		fmt.Fprint(out, help)
		return 0
	}
	command := args[0]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	// flag can print raw argument values. User-facing parse errors are deliberately static.
	flags.SetOutput(io.Discard)
	var asJSON, openBrowser bool
	var mib, samples int
	var profile string
	switch command {
	case "help", "-h", "--help":
		fmt.Fprint(out, help)
		return 0
	case "version", "--version":
		if len(args) != 1 {
			fmt.Fprintln(errOut, "Kullanım  afa version")
			return 2
		}
		fmt.Fprintf(out, "AFA %s (%s/%s) · Go %s · %s\n",
			buildinfo.Version, runtime.GOOS, runtime.GOARCH, runtime.Version(), buildinfo.Commit)
		return 0
	case "portal":
		flags.BoolVar(&openBrowser, "open", false, "Yoklamadan sonra varsayılan tarayıcıyı aç")
	case "check":
		flags.StringVar(&profile, "profile", "", "Belgelenmiş web adresi profili")
	case "speed":
		flags.IntVar(&mib, "mib", speed.DefaultMiB, "Her örnekte indirilecek veri (1–8 MiB)")
		flags.IntVar(&samples, "samples", 1, "İndirme örneği sayısı (1–5)")
	case "quality":
		flags.IntVar(&samples, "samples", speed.DefaultQualitySamples, "Küçük HTTPS yoklaması sayısı (2–8)")
	default:
		fmt.Fprintln(errOut, "Komut tanınmadı. Komutları görmek için afa help çalıştırın.")
		return 2
	}
	flags.BoolVar(&asJSON, "json", false, "Sonucu JSON olarak yaz")
	parseErr := flags.Parse(args[1:])
	flags.SetOutput(errOut)
	if errors.Is(parseErr, flag.ErrHelp) {
		fmt.Fprint(out, help)
		flags.SetOutput(out)
		flags.PrintDefaults()
		return 0
	}
	wantedArgs := 0
	if command == "check" && profile == "" {
		wantedArgs = 1
	}
	if parseErr != nil || flags.NArg() != wantedArgs {
		fmt.Fprintln(errOut, "Geçersiz komut seçenekleri.")
		flags.PrintDefaults()
		return 2
	}

	started := time.Now()
	var result outcome
	switch command {
	case "portal":
		result = runPortal(ctx, started, openBrowser, errOut, ops)
	case "check":
		if profile != "" {
			result = runProfile(ctx, started, profile, ops)
		} else {
			result = runCheck(ctx, started, flags.Arg(0), ops)
		}
	case "speed":
		if mib < 1 || mib > speed.MaxMiB || samples < 1 || samples > 5 {
			result = invalid(command, started, "Örnek başına 1–8 MiB ve 1–5 örnek seçin.")
		} else {
			result = runSpeed(ctx, started, mib, samples, ops)
		}
	case "quality":
		if samples < 2 || samples > speed.MaxQualitySamples {
			result = invalid(command, started, "2–8 HTTPS yoklaması seçin.")
		} else {
			result = runQuality(ctx, started, samples, ops)
		}
	}
	if asJSON {
		if err := json.NewEncoder(out).Encode(result.value); err != nil {
			fmt.Fprintln(errOut, "Sonuç yazılamadı.")
			return 1
		}
	} else {
		target := out
		if result.code == 2 || result.toStderr {
			target = errOut
		}
		if _, err := fmt.Fprintln(target, result.text); err != nil {
			return 1
		}
	}
	return result.code
}
