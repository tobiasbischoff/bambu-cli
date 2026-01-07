package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bambu-cli/internal/config"
	"bambu-cli/internal/output"
	"bambu-cli/internal/printer"
	"bambu-cli/internal/ui"
)

var version = "dev"

type GlobalFlags struct {
	Help            bool
	Version         bool
	Quiet           bool
	Verbose         bool
	JSON            bool
	Plain           bool
	NoColor         bool
	NoInput         bool
	Force           bool
	Confirm         string
	DryRun          bool
	Printer         string
	IP              string
	Serial          string
	AccessCodeFile  string
	AccessCodeStdin bool
	NoCamera        bool
	TimeoutSeconds  int
	ConfigPath      string
}

type ResolvedPrinter struct {
	IP             string
	Serial         string
	AccessCode     string
	Username       string
	MQTTPort       int
	FTPPort        int
	CameraPort     int
	Timeout        time.Duration
	NoCamera       bool
	ProfileName    string
	ConfigPathUsed string
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	gf, rest, err := parseGlobalFlags(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 2
	}
	if gf.Version {
		fmt.Fprintln(os.Stdout, version)
		return 0
	}
	if gf.Help || len(rest) == 0 {
		printUsage()
		if gf.Help {
			return 0
		}
		return 2
	}

	cmd := rest[0]
	subargs := rest[1:]

	switch cmd {
	case "help":
		return cmdHelp(gf, subargs)
	case "status":
		return cmdStatus(gf, subargs)
	case "watch":
		return cmdWatch(gf, subargs)
	case "light":
		return cmdLight(gf, subargs)
	case "temps":
		return cmdTemps(gf, subargs)
	case "print":
		return cmdPrint(gf, subargs)
	case "files":
		return cmdFiles(gf, subargs)
	case "camera":
		return cmdCamera(gf, subargs)
	case "gcode":
		return cmdGcode(gf, subargs)
	case "ams":
		return cmdAMS(gf, subargs)
	case "calibrate":
		return cmdCalibrate(gf, subargs)
	case "home":
		return cmdHome(gf, subargs)
	case "move":
		return cmdMove(gf, subargs)
	case "fans":
		return cmdFans(gf, subargs)
	case "reboot":
		return cmdReboot(gf, subargs)
	case "config":
		return cmdConfig(gf, subargs)
	case "doctor":
		return cmdDoctor(gf, subargs)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		return 2
	}
}

func parseGlobalFlags(args []string) (GlobalFlags, []string, error) {
	fs := flag.NewFlagSet("bambu-cli", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var gf GlobalFlags
	fs.BoolVar(&gf.Help, "help", false, "show help")
	fs.BoolVar(&gf.Help, "h", false, "show help")
	fs.BoolVar(&gf.Version, "version", false, "show version")
	fs.BoolVar(&gf.Quiet, "quiet", false, "less output")
	fs.BoolVar(&gf.Quiet, "q", false, "less output")
	fs.BoolVar(&gf.Verbose, "verbose", false, "more output")
	fs.BoolVar(&gf.Verbose, "v", false, "more output")
	fs.BoolVar(&gf.JSON, "json", false, "json output")
	fs.BoolVar(&gf.Plain, "plain", false, "plain output")
	fs.BoolVar(&gf.NoColor, "no-color", false, "disable color")
	fs.BoolVar(&gf.NoInput, "no-input", false, "disable prompts")
	fs.BoolVar(&gf.Force, "force", false, "skip confirmation")
	fs.BoolVar(&gf.Force, "f", false, "skip confirmation")
	fs.StringVar(&gf.Confirm, "confirm", "", "confirmation token")
	fs.BoolVar(&gf.DryRun, "dry-run", false, "preview only")
	fs.BoolVar(&gf.DryRun, "n", false, "preview only")
	fs.StringVar(&gf.Printer, "printer", "", "printer profile")
	fs.StringVar(&gf.IP, "ip", "", "printer IP")
	fs.StringVar(&gf.Serial, "serial", "", "printer serial")
	fs.StringVar(&gf.AccessCodeFile, "access-code-file", "", "path to access code file")
	fs.BoolVar(&gf.AccessCodeStdin, "access-code-stdin", false, "read access code from stdin")
	fs.BoolVar(&gf.NoCamera, "no-camera", false, "skip camera connection")
	fs.IntVar(&gf.TimeoutSeconds, "timeout", 0, "network timeout in seconds")
	fs.StringVar(&gf.ConfigPath, "config", "", "config file path")

	if err := fs.Parse(args); err != nil {
		return gf, nil, err
	}
	if gf.JSON && gf.Plain {
		return gf, nil, errors.New("--json and --plain are mutually exclusive")
	}
	return gf, fs.Args(), nil
}

func resolvePrinter(gf GlobalFlags, needAccess bool, needSerial bool) (ResolvedPrinter, error) {
	cwd, _ := os.Getwd()
	projectPath := config.ProjectConfigPath(cwd)
	userPath, err := config.UserConfigPath()
	if err != nil {
		return ResolvedPrinter{}, err
	}

	userCfgPath := userPath
	if gf.ConfigPath != "" {
		userCfgPath = gf.ConfigPath
	}

	userCfg, err := config.Read(userCfgPath)
	if err != nil {
		return ResolvedPrinter{}, err
	}
	projectCfg, err := config.Read(projectPath)
	if err != nil {
		return ResolvedPrinter{}, err
	}
	cfg := config.Merge(userCfg, projectCfg)

	profileName := firstNonEmpty(gf.Printer, os.Getenv("BAMBU_PROFILE"), cfg.DefaultProfile)
	if profileName == "" && len(cfg.Profiles) == 1 {
		for k := range cfg.Profiles {
			profileName = k
		}
	}
	profile := config.Profile{}
	if profileName != "" {
		if p, ok := cfg.Profiles[profileName]; ok {
			profile = p
		}
	}

	res := ResolvedPrinter{
		IP:             firstNonEmpty(gf.IP, os.Getenv("BAMBU_IP"), profile.IP),
		Serial:         firstNonEmpty(gf.Serial, os.Getenv("BAMBU_SERIAL"), profile.Serial),
		AccessCode:     "",
		Username:       firstNonEmpty(profile.Username, "bblp"),
		MQTTPort:       firstNonZero(envInt("BAMBU_MQTT_PORT"), profile.MQTTPort, 8883),
		FTPPort:        firstNonZero(envInt("BAMBU_FTP_PORT"), profile.FTPPort, 990),
		CameraPort:     firstNonZero(envInt("BAMBU_CAMERA_PORT"), profile.CameraPort, 6000),
		Timeout:        time.Duration(firstNonZero(gf.TimeoutSeconds, envInt("BAMBU_TIMEOUT"), profile.TimeoutSeconds, 10)) * time.Second,
		NoCamera:       gf.NoCamera || envBool("BAMBU_NO_CAMERA") || profile.NoCamera,
		ProfileName:    profileName,
		ConfigPathUsed: userCfgPath,
	}

	accessFile := firstNonEmpty(gf.AccessCodeFile, os.Getenv("BAMBU_ACCESS_CODE_FILE"), profile.AccessCodeFile)
	if needAccess {
		code, err := resolveAccessCode(accessFile, gf.AccessCodeStdin)
		if err != nil {
			return ResolvedPrinter{}, err
		}
		res.AccessCode = code
	}

	if res.IP == "" {
		return ResolvedPrinter{}, errors.New("missing printer IP; use --ip or config")
	}
	if needSerial && res.Serial == "" {
		return ResolvedPrinter{}, errors.New("missing printer serial; use --serial or config")
	}
	if needAccess && res.AccessCode == "" {
		return ResolvedPrinter{}, errors.New("missing access code; use --access-code-file or --access-code-stdin")
	}

	return res, nil
}

func resolveAccessCode(path string, fromStdin bool) (string, error) {
	if fromStdin {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		code := strings.TrimSpace(string(data))
		if code == "" {
			return "", errors.New("access code from stdin is empty")
		}
		return code, nil
	}
	if path == "" {
		return "", errors.New("access code file not set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	code := strings.TrimSpace(string(data))
	if code == "" {
		return "", errors.New("access code file is empty")
	}
	return code, nil
}

func cmdHelp(_ GlobalFlags, args []string) int {
	if len(args) == 0 {
		printUsage()
		return 0
	}
	printCommandUsage(args[0])
	return 0
}

func cmdStatus(gf GlobalFlags, _ []string) int {
	res, err := resolvePrinter(gf, true, true)
	if err != nil {
		return errExit(err)
	}
	client, err := printer.NewMQTTClient(res.IP, res.AccessCode, res.Serial, res.Username, res.MQTTPort, res.Timeout)
	if err != nil {
		return errExit(err)
	}
	defer client.Close()

	_ = client.PushAll()
	_ = client.WaitForData(res.Timeout)

	status := printer.GetStatus(client)

	format := selectFormat(gf)
	switch format {
	case output.JSON:
		if err := output.WriteJSON(os.Stdout, status); err != nil {
			return errExit(err)
		}
	case output.Plain:
		kv := map[string]string{
			"gcode_state":       string(status.GcodeState),
			"print_status":      status.PrintStatus,
			"percent":           strconv.Itoa(status.Percent),
			"layer_current":     strconv.Itoa(status.LayerCurrent),
			"layer_total":       strconv.Itoa(status.LayerTotal),
			"bed_temp":          fmtFloat(status.BedTemp),
			"nozzle_temp":       fmtFloat(status.NozzleTemp),
			"chamber_temp":      fmtFloat(status.ChamberTemp),
			"file":              status.File,
			"light":             status.Light,
			"wifi_signal":       status.WifiSignal,
			"error_code":        strconv.Itoa(status.ErrorCode),
			"remaining_minutes": formatRemaining(status.RemainingMinutes),
		}
		if err := output.WritePlainKV(os.Stdout, kv); err != nil {
			return errExit(err)
		}
	default:
		printStatusHuman(status)
	}

	return 0
}

func cmdWatch(gf GlobalFlags, args []string) int {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	interval := fs.Int("interval", 5, "seconds between updates")
	refresh := fs.Bool("refresh", false, "send pushall each interval")
	if err := fs.Parse(args); err != nil {
		return errExit(err)
	}

	res, err := resolvePrinter(gf, true, true)
	if err != nil {
		return errExit(err)
	}
	client, err := printer.NewMQTTClient(res.IP, res.AccessCode, res.Serial, res.Username, res.MQTTPort, res.Timeout)
	if err != nil {
		return errExit(err)
	}
	defer client.Close()

	_ = client.PushAll()
	_ = client.WaitForData(res.Timeout)

	ticker := time.NewTicker(time.Duration(*interval) * time.Second)
	defer ticker.Stop()

	for {
		status := printer.GetStatus(client)
		format := selectFormat(gf)
		switch format {
		case output.JSON:
			if err := output.WriteJSON(os.Stdout, status); err != nil {
				return errExit(err)
			}
		case output.Plain:
			kv := map[string]string{
				"timestamp":         time.Now().Format(time.RFC3339),
				"gcode_state":       string(status.GcodeState),
				"print_status":      status.PrintStatus,
				"percent":           strconv.Itoa(status.Percent),
				"layer_current":     strconv.Itoa(status.LayerCurrent),
				"layer_total":       strconv.Itoa(status.LayerTotal),
				"bed_temp":          fmtFloat(status.BedTemp),
				"nozzle_temp":       fmtFloat(status.NozzleTemp),
				"chamber_temp":      fmtFloat(status.ChamberTemp),
				"file":              status.File,
				"light":             status.Light,
				"wifi_signal":       status.WifiSignal,
				"error_code":        strconv.Itoa(status.ErrorCode),
				"remaining_minutes": formatRemaining(status.RemainingMinutes),
			}
			if err := output.WritePlainKV(os.Stdout, kv); err != nil {
				return errExit(err)
			}
		default:
			printStatusHuman(status)
		}

		<-ticker.C
		if *refresh {
			_ = client.PushAll()
		}
	}
}

func cmdLight(gf GlobalFlags, args []string) int {
	if len(args) == 0 {
		printCommandUsage("light")
		return 2
	}
	res, err := resolvePrinter(gf, true, true)
	if err != nil {
		return errExit(err)
	}

	action := args[0]
	if action == "status" {
		return cmdStatus(gf, nil)
	}

	if gf.DryRun {
		fmt.Fprintf(os.Stdout, "Would set light %s\n", action)
		return 0
	}

	client, err := printer.NewMQTTClient(res.IP, res.AccessCode, res.Serial, res.Username, res.MQTTPort, res.Timeout)
	if err != nil {
		return errExit(err)
	}
	defer client.Close()

	switch action {
	case "on":
		return exitOnErr(client.Publish(printer.PayloadLight(true)))
	case "off":
		return exitOnErr(client.Publish(printer.PayloadLight(false)))
	default:
		printCommandUsage("light")
		return 2
	}
}

func cmdTemps(gf GlobalFlags, args []string) int {
	if len(args) == 0 {
		printCommandUsage("temps")
		return 2
	}
	sub := args[0]
	subargs := args[1:]

	switch sub {
	case "get":
		return cmdTempsGet(gf, subargs)
	case "set":
		return cmdTempsSet(gf, subargs)
	default:
		printCommandUsage("temps")
		return 2
	}
}

func cmdTempsGet(gf GlobalFlags, _ []string) int {
	res, err := resolvePrinter(gf, true, true)
	if err != nil {
		return errExit(err)
	}
	client, err := printer.NewMQTTClient(res.IP, res.AccessCode, res.Serial, res.Username, res.MQTTPort, res.Timeout)
	if err != nil {
		return errExit(err)
	}
	defer client.Close()
	_ = client.PushAll()
	_ = client.WaitForData(res.Timeout)

	status := printer.GetStatus(client)
	format := selectFormat(gf)
	switch format {
	case output.JSON:
		return exitOnErr(output.WriteJSON(os.Stdout, map[string]float64{
			"bed":     status.BedTemp,
			"nozzle":  status.NozzleTemp,
			"chamber": status.ChamberTemp,
		}))
	case output.Plain:
		return exitOnErr(output.WritePlainKV(os.Stdout, map[string]string{
			"bed":     fmtFloat(status.BedTemp),
			"nozzle":  fmtFloat(status.NozzleTemp),
			"chamber": fmtFloat(status.ChamberTemp),
		}))
	default:
		fmt.Fprintf(os.Stdout, "Bed: %s C\nNozzle: %s C\nChamber: %s C\n", fmtFloat(status.BedTemp), fmtFloat(status.NozzleTemp), fmtFloat(status.ChamberTemp))
		return 0
	}
}

func cmdTempsSet(gf GlobalFlags, args []string) int {
	fs := flag.NewFlagSet("temps set", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	bed := fs.Int("bed", -1, "bed temp C")
	nozzle := fs.Int("nozzle", -1, "nozzle temp C")
	chamber := fs.Int("chamber", -1, "chamber temp C")
	if err := fs.Parse(args); err != nil {
		return errExit(err)
	}
	if *bed < 0 && *nozzle < 0 && *chamber < 0 {
		return errExit(errors.New("set at least one temperature"))
	}
	if gf.DryRun {
		fmt.Fprintf(os.Stdout, "Would set temperatures bed=%d nozzle=%d chamber=%d\n", *bed, *nozzle, *chamber)
		return 0
	}

	res, err := resolvePrinter(gf, true, true)
	if err != nil {
		return errExit(err)
	}
	client, err := printer.NewMQTTClient(res.IP, res.AccessCode, res.Serial, res.Username, res.MQTTPort, res.Timeout)
	if err != nil {
		return errExit(err)
	}
	defer client.Close()

	lines := []string{}
	if *bed >= 0 {
		lines = append(lines, fmt.Sprintf("M140 S%d", *bed))
	}
	if *nozzle >= 0 {
		lines = append(lines, fmt.Sprintf("M104 S%d", *nozzle))
	}
	if *chamber >= 0 {
		lines = append(lines, fmt.Sprintf("M141 S%d", *chamber))
	}
	payload := printer.PayloadGcode(strings.Join(lines, "\n"))
	return exitOnErr(client.Publish(payload))
}

func cmdPrint(gf GlobalFlags, args []string) int {
	if len(args) == 0 {
		printCommandUsage("print")
		return 2
	}
	sub := args[0]
	subargs := args[1:]

	switch sub {
	case "start":
		return cmdPrintStart(gf, subargs)
	case "pause":
		return cmdPrintPause(gf, subargs)
	case "resume":
		return cmdPrintResume(gf, subargs)
	case "stop":
		return cmdPrintStop(gf, subargs)
	default:
		printCommandUsage("print")
		return 2
	}
}

func cmdPrintStart(gf GlobalFlags, args []string) int {
	fs := flag.NewFlagSet("print start", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	plate := fs.String("plate", "1", "plate number or gcode path")
	noUpload := fs.Bool("no-upload", false, "do not upload file")
	noAMS := fs.Bool("no-ams", false, "disable AMS")
	amsMapping := fs.String("ams-mapping", "0", "comma-separated AMS mapping")
	skipObjects := fs.String("skip-objects", "", "comma-separated object IDs")
	flowCalibration := fs.Bool("flow-calibration", true, "enable flow calibration")
	remoteName := fs.String("remote-name", "", "remote filename")
	if err := fs.Parse(args); err != nil {
		return errExit(err)
	}
	if fs.NArg() < 1 {
		return errExit(errors.New("print start requires a file path"))
	}

	inputPath := fs.Arg(0)
	if gf.DryRun {
		fmt.Fprintf(os.Stdout, "Would start print for %s\n", inputPath)
		return 0
	}

	res, err := resolvePrinter(gf, true, true)
	if err != nil {
		return errExit(err)
	}

	plateLocation := plateToLocation(*plate)
	useAMS := !*noAMS
	mapping, err := parseIntList(*amsMapping)
	if err != nil {
		return errExit(err)
	}
	var skipList []int
	if *skipObjects != "" {
		skipList, err = parseIntList(*skipObjects)
		if err != nil {
			return errExit(err)
		}
	}

	remote := *remoteName
	if remote == "" {
		if *noUpload {
			remote = inputPath
		} else {
			remote = defaultRemoteName(inputPath)
		}
	}

	if *noUpload {
		if strings.HasSuffix(strings.ToLower(inputPath), ".gcode") {
			return errExit(errors.New("--no-upload cannot be used with .gcode input"))
		}
	} else {
		ftpClient := printer.NewFTPClient(res.IP, res.AccessCode, res.Username, res.FTPPort, res.Timeout)
		if strings.HasSuffix(strings.ToLower(inputPath), ".3mf") {
			if err := ftpClient.Upload(inputPath, remote); err != nil {
				return errExit(err)
			}
		} else {
			tmpPath, cleanup, err := printer.Create3MFTempFromFile(inputPath, plateLocation)
			if err != nil {
				return errExit(err)
			}
			defer cleanup()
			if err := ftpClient.Upload(tmpPath, remote); err != nil {
				return errExit(err)
			}
		}
	}

	client, err := printer.NewMQTTClient(res.IP, res.AccessCode, res.Serial, res.Username, res.MQTTPort, res.Timeout)
	if err != nil {
		return errExit(err)
	}
	defer client.Close()

	payload := printer.PayloadStartPrint(remote, plateLocation, useAMS, mapping, skipList, *flowCalibration)
	return exitOnErr(client.Publish(payload))
}

func cmdPrintPause(gf GlobalFlags, _ []string) int {
	res, err := resolvePrinter(gf, true, true)
	if err != nil {
		return errExit(err)
	}
	client, err := printer.NewMQTTClient(res.IP, res.AccessCode, res.Serial, res.Username, res.MQTTPort, res.Timeout)
	if err != nil {
		return errExit(err)
	}
	defer client.Close()
	return exitOnErr(client.Publish(printer.PayloadPrintPause()))
}

func cmdPrintResume(gf GlobalFlags, _ []string) int {
	res, err := resolvePrinter(gf, true, true)
	if err != nil {
		return errExit(err)
	}
	client, err := printer.NewMQTTClient(res.IP, res.AccessCode, res.Serial, res.Username, res.MQTTPort, res.Timeout)
	if err != nil {
		return errExit(err)
	}
	defer client.Close()
	return exitOnErr(client.Publish(printer.PayloadPrintResume()))
}

func cmdPrintStop(gf GlobalFlags, _ []string) int {
	if err := ui.RequireConfirmation(ui.ConfirmOptions{
		Action:  "stop",
		Force:   gf.Force,
		Confirm: gf.Confirm,
		NoInput: gf.NoInput,
		UseTTY:  ui.IsTerminal(os.Stdin),
		Out:     os.Stderr,
	}); err != nil {
		return errExit(err)
	}
	if gf.DryRun {
		fmt.Fprintln(os.Stdout, "Would stop print")
		return 0
	}

	res, err := resolvePrinter(gf, true, true)
	if err != nil {
		return errExit(err)
	}
	client, err := printer.NewMQTTClient(res.IP, res.AccessCode, res.Serial, res.Username, res.MQTTPort, res.Timeout)
	if err != nil {
		return errExit(err)
	}
	defer client.Close()
	return exitOnErr(client.Publish(printer.PayloadPrintStop()))
}

func cmdFiles(gf GlobalFlags, args []string) int {
	if len(args) == 0 {
		printCommandUsage("files")
		return 2
	}
	sub := args[0]
	subargs := args[1:]

	switch sub {
	case "list":
		return cmdFilesList(gf, subargs)
	case "upload":
		return cmdFilesUpload(gf, subargs)
	case "download":
		return cmdFilesDownload(gf, subargs)
	case "delete":
		return cmdFilesDelete(gf, subargs)
	default:
		printCommandUsage("files")
		return 2
	}
}

func cmdFilesList(gf GlobalFlags, args []string) int {
	fs := flag.NewFlagSet("files list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dir := fs.String("dir", "", "directory to list")
	if err := fs.Parse(args); err != nil {
		return errExit(err)
	}

	res, err := resolvePrinter(gf, true, false)
	if err != nil {
		return errExit(err)
	}
	ftpClient := printer.NewFTPClient(res.IP, res.AccessCode, res.Username, res.FTPPort, res.Timeout)
	entries, err := ftpClient.List(*dir)
	if err != nil {
		return errExit(err)
	}

	format := selectFormat(gf)
	switch format {
	case output.JSON:
		return exitOnErr(output.WriteJSON(os.Stdout, map[string]any{"entries": entries}))
	case output.Plain:
		for _, e := range entries {
			fmt.Fprintln(os.Stdout, e)
		}
		return 0
	default:
		for _, e := range entries {
			fmt.Fprintln(os.Stdout, e)
		}
		return 0
	}
}

func cmdFilesUpload(gf GlobalFlags, args []string) int {
	fs := flag.NewFlagSet("files upload", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	remote := fs.String("as", "", "remote filename")
	if err := fs.Parse(args); err != nil {
		return errExit(err)
	}
	if fs.NArg() < 1 {
		return errExit(errors.New("files upload requires a local path"))
	}
	localPath := fs.Arg(0)

	if gf.DryRun {
		fmt.Fprintf(os.Stdout, "Would upload %s\n", localPath)
		return 0
	}

	res, err := resolvePrinter(gf, true, false)
	if err != nil {
		return errExit(err)
	}
	ftpClient := printer.NewFTPClient(res.IP, res.AccessCode, res.Username, res.FTPPort, res.Timeout)
	remotePath := *remote
	if remotePath == "" {
		remotePath = filepath.Base(localPath)
	}
	return exitOnErr(ftpClient.Upload(localPath, remotePath))
}

func cmdFilesDownload(gf GlobalFlags, args []string) int {
	fs := flag.NewFlagSet("files download", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	outPath := fs.String("out", "", "output file path or - for stdout")
	if err := fs.Parse(args); err != nil {
		return errExit(err)
	}
	if fs.NArg() < 1 {
		return errExit(errors.New("files download requires a remote path"))
	}
	remotePath := fs.Arg(0)

	res, err := resolvePrinter(gf, true, false)
	if err != nil {
		return errExit(err)
	}
	ftpClient := printer.NewFTPClient(res.IP, res.AccessCode, res.Username, res.FTPPort, res.Timeout)

	var w io.Writer
	var file *os.File
	if *outPath == "" {
		return errExit(errors.New("--out is required"))
	}
	if *outPath == "-" {
		if ui.IsTerminal(os.Stdout) && !gf.Force {
			return errExit(errors.New("refusing to write binary data to terminal; use --force or --out <file>"))
		}
		w = os.Stdout
	} else {
		file, err = os.Create(*outPath)
		if err != nil {
			return errExit(err)
		}
		defer file.Close()
		w = file
	}

	return exitOnErr(ftpClient.Download(remotePath, w))
}

func cmdFilesDelete(gf GlobalFlags, args []string) int {
	fs := flag.NewFlagSet("files delete", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return errExit(err)
	}
	if fs.NArg() < 1 {
		return errExit(errors.New("files delete requires a remote path"))
	}
	remotePath := fs.Arg(0)

	if err := ui.RequireConfirmation(ui.ConfirmOptions{
		Action:  "delete",
		Force:   gf.Force,
		Confirm: gf.Confirm,
		NoInput: gf.NoInput,
		UseTTY:  ui.IsTerminal(os.Stdin),
		Out:     os.Stderr,
	}); err != nil {
		return errExit(err)
	}
	if gf.DryRun {
		fmt.Fprintf(os.Stdout, "Would delete %s\n", remotePath)
		return 0
	}

	res, err := resolvePrinter(gf, true, false)
	if err != nil {
		return errExit(err)
	}
	ftpClient := printer.NewFTPClient(res.IP, res.AccessCode, res.Username, res.FTPPort, res.Timeout)
	return exitOnErr(ftpClient.Delete(remotePath))
}

func cmdCamera(gf GlobalFlags, args []string) int {
	if len(args) == 0 || args[0] != "snapshot" {
		printCommandUsage("camera")
		return 2
	}
	fs := flag.NewFlagSet("camera snapshot", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	outPath := fs.String("out", "snapshot.jpg", "output file path or - for stdout")
	if err := fs.Parse(args[1:]); err != nil {
		return errExit(err)
	}

	if gf.DryRun {
		fmt.Fprintf(os.Stdout, "Would take snapshot to %s\n", *outPath)
		return 0
	}

	res, err := resolvePrinter(gf, true, false)
	if err != nil {
		return errExit(err)
	}
	cam := printer.NewCameraClient(res.IP, res.AccessCode, res.Username, res.CameraPort, res.Timeout)
	img, err := cam.Snapshot()
	if err != nil {
		return errExit(err)
	}

	if *outPath == "-" {
		if ui.IsTerminal(os.Stdout) && !gf.Force {
			return errExit(errors.New("refusing to write binary data to terminal; use --force or --out <file>"))
		}
		_, err = os.Stdout.Write(img)
		return exitOnErr(err)
	}
	return exitOnErr(os.WriteFile(*outPath, img, 0o644))
}

func cmdGcode(gf GlobalFlags, args []string) int {
	if len(args) == 0 || args[0] != "send" {
		printCommandUsage("gcode")
		return 2
	}
	fs := flag.NewFlagSet("gcode send", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	stdin := fs.Bool("stdin", false, "read gcode lines from stdin")
	noCheck := fs.Bool("no-check", false, "skip gcode validation")
	if err := fs.Parse(args[1:]); err != nil {
		return errExit(err)
	}

	if err := ui.RequireConfirmation(ui.ConfirmOptions{
		Action:  "gcode",
		Force:   gf.Force,
		Confirm: gf.Confirm,
		NoInput: gf.NoInput,
		UseTTY:  ui.IsTerminal(os.Stdin),
		Out:     os.Stderr,
	}); err != nil {
		return errExit(err)
	}

	var lines []string
	if *stdin {
		if gf.AccessCodeStdin {
			return errExit(errors.New("cannot use --access-code-stdin with gcode --stdin"))
		}
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			lines = append(lines, line)
		}
		if err := scanner.Err(); err != nil {
			return errExit(err)
		}
	} else {
		if fs.NArg() == 0 {
			return errExit(errors.New("gcode send requires at least one line or --stdin"))
		}
		lines = fs.Args()
	}

	if !*noCheck {
		for _, line := range lines {
			if !printer.ValidateGcodeLine(line) {
				return errExit(fmt.Errorf("invalid gcode line: %s", line))
			}
		}
	}

	if gf.DryRun {
		fmt.Fprintf(os.Stdout, "Would send %d gcode line(s)\n", len(lines))
		return 0
	}

	res, err := resolvePrinter(gf, true, true)
	if err != nil {
		return errExit(err)
	}
	client, err := printer.NewMQTTClient(res.IP, res.AccessCode, res.Serial, res.Username, res.MQTTPort, res.Timeout)
	if err != nil {
		return errExit(err)
	}
	defer client.Close()

	payload := printer.PayloadGcode(strings.Join(lines, "\n"))
	return exitOnErr(client.Publish(payload))
}

func cmdAMS(gf GlobalFlags, args []string) int {
	if len(args) == 0 || args[0] != "status" {
		printCommandUsage("ams")
		return 2
	}
	res, err := resolvePrinter(gf, true, true)
	if err != nil {
		return errExit(err)
	}
	client, err := printer.NewMQTTClient(res.IP, res.AccessCode, res.Serial, res.Username, res.MQTTPort, res.Timeout)
	if err != nil {
		return errExit(err)
	}
	defer client.Close()

	_ = client.PushAll()
	_ = client.WaitForData(res.Timeout)

	amsInfo, _ := client.Get("print", "ams")
	format := selectFormat(gf)
	switch format {
	case output.JSON:
		return exitOnErr(output.WriteJSON(os.Stdout, map[string]any{"ams": amsInfo}))
	case output.Plain:
		return exitOnErr(writeAMSPlain(os.Stdout, amsInfo))
	default:
		return exitOnErr(writeAMSHuman(os.Stdout, amsInfo))
	}
}

func cmdCalibrate(gf GlobalFlags, args []string) int {
	fs := flag.NewFlagSet("calibrate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	noBed := fs.Bool("no-bed-level", false, "disable bed leveling")
	noMotor := fs.Bool("no-motor-noise", false, "disable motor noise calibration")
	noVibration := fs.Bool("no-vibration", false, "disable vibration compensation")
	if err := fs.Parse(args); err != nil {
		return errExit(err)
	}

	if err := ui.RequireConfirmation(ui.ConfirmOptions{
		Action:  "calibrate",
		Force:   gf.Force,
		Confirm: gf.Confirm,
		NoInput: gf.NoInput,
		UseTTY:  ui.IsTerminal(os.Stdin),
		Out:     os.Stderr,
	}); err != nil {
		return errExit(err)
	}

	if gf.DryRun {
		fmt.Fprintln(os.Stdout, "Would start calibration")
		return 0
	}

	res, err := resolvePrinter(gf, true, true)
	if err != nil {
		return errExit(err)
	}
	client, err := printer.NewMQTTClient(res.IP, res.AccessCode, res.Serial, res.Username, res.MQTTPort, res.Timeout)
	if err != nil {
		return errExit(err)
	}
	defer client.Close()

	payload := printer.PayloadCalibration(!*noBed, !*noMotor, !*noVibration)
	return exitOnErr(client.Publish(payload))
}

func cmdHome(gf GlobalFlags, _ []string) int {
	if gf.DryRun {
		fmt.Fprintln(os.Stdout, "Would home printer")
		return 0
	}
	res, err := resolvePrinter(gf, true, true)
	if err != nil {
		return errExit(err)
	}
	client, err := printer.NewMQTTClient(res.IP, res.AccessCode, res.Serial, res.Username, res.MQTTPort, res.Timeout)
	if err != nil {
		return errExit(err)
	}
	defer client.Close()
	payload := printer.PayloadGcode("G28")
	return exitOnErr(client.Publish(payload))
}

func cmdMove(gf GlobalFlags, args []string) int {
	if len(args) == 0 || args[0] != "z" {
		printCommandUsage("move")
		return 2
	}
	fs := flag.NewFlagSet("move z", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	height := fs.Int("height", -1, "z height (0-256)")
	if err := fs.Parse(args[1:]); err != nil {
		return errExit(err)
	}
	if *height < 0 {
		return errExit(errors.New("--height is required"))
	}
	if gf.DryRun {
		fmt.Fprintf(os.Stdout, "Would move Z to %d\n", *height)
		return 0
	}

	res, err := resolvePrinter(gf, true, true)
	if err != nil {
		return errExit(err)
	}
	client, err := printer.NewMQTTClient(res.IP, res.AccessCode, res.Serial, res.Username, res.MQTTPort, res.Timeout)
	if err != nil {
		return errExit(err)
	}
	defer client.Close()

	payload := printer.PayloadGcode(fmt.Sprintf("G90\nG0 Z%d", *height))
	return exitOnErr(client.Publish(payload))
}

func cmdFans(gf GlobalFlags, args []string) int {
	if len(args) == 0 || args[0] != "set" {
		printCommandUsage("fans")
		return 2
	}
	fs := flag.NewFlagSet("fans set", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	part := fs.String("part", "", "part fan speed (0-255 or 0-1)")
	aux := fs.String("aux", "", "aux fan speed (0-255 or 0-1)")
	chamber := fs.String("chamber", "", "chamber fan speed (0-255 or 0-1)")
	if err := fs.Parse(args[1:]); err != nil {
		return errExit(err)
	}
	if *part == "" && *aux == "" && *chamber == "" {
		return errExit(errors.New("set at least one fan speed"))
	}

	lines := []string{}
	if *part != "" {
		val, err := parseFan(*part)
		if err != nil {
			return errExit(err)
		}
		lines = append(lines, fmt.Sprintf("M106 P1 S%d", val))
	}
	if *aux != "" {
		val, err := parseFan(*aux)
		if err != nil {
			return errExit(err)
		}
		lines = append(lines, fmt.Sprintf("M106 P2 S%d", val))
	}
	if *chamber != "" {
		val, err := parseFan(*chamber)
		if err != nil {
			return errExit(err)
		}
		lines = append(lines, fmt.Sprintf("M106 P3 S%d", val))
	}

	if gf.DryRun {
		fmt.Fprintln(os.Stdout, "Would set fan speeds")
		return 0
	}

	res, err := resolvePrinter(gf, true, true)
	if err != nil {
		return errExit(err)
	}
	client, err := printer.NewMQTTClient(res.IP, res.AccessCode, res.Serial, res.Username, res.MQTTPort, res.Timeout)
	if err != nil {
		return errExit(err)
	}
	defer client.Close()

	payload := printer.PayloadGcode(strings.Join(lines, "\n"))
	return exitOnErr(client.Publish(payload))
}

func cmdReboot(gf GlobalFlags, _ []string) int {
	if err := ui.RequireConfirmation(ui.ConfirmOptions{
		Action:  "reboot",
		Force:   gf.Force,
		Confirm: gf.Confirm,
		NoInput: gf.NoInput,
		UseTTY:  ui.IsTerminal(os.Stdin),
		Out:     os.Stderr,
	}); err != nil {
		return errExit(err)
	}
	if gf.DryRun {
		fmt.Fprintln(os.Stdout, "Would reboot printer")
		return 0
	}
	res, err := resolvePrinter(gf, true, true)
	if err != nil {
		return errExit(err)
	}
	client, err := printer.NewMQTTClient(res.IP, res.AccessCode, res.Serial, res.Username, res.MQTTPort, res.Timeout)
	if err != nil {
		return errExit(err)
	}
	defer client.Close()
	return exitOnErr(client.Publish(printer.PayloadReboot()))
}

func cmdConfig(gf GlobalFlags, args []string) int {
	if len(args) == 0 {
		printCommandUsage("config")
		return 2
	}
	sub := args[0]
	subargs := args[1:]

	switch sub {
	case "list":
		return cmdConfigList(gf)
	case "get":
		return cmdConfigGet(gf, subargs)
	case "set":
		return cmdConfigSet(gf, subargs)
	case "remove":
		return cmdConfigRemove(gf, subargs)
	default:
		printCommandUsage("config")
		return 2
	}
}

func cmdConfigList(gf GlobalFlags) int {
	cfgPath, cfg, err := loadConfigForEdit(gf)
	if err != nil {
		return errExit(err)
	}
	format := selectFormat(gf)
	if format == output.JSON {
		return exitOnErr(output.WriteJSON(os.Stdout, cfg))
	}
	fmt.Fprintf(os.Stdout, "Config file: %s\n", cfgPath)
	return exitOnErr(output.WriteJSON(os.Stdout, cfg))
}

func cmdConfigGet(gf GlobalFlags, args []string) int {
	fs := flag.NewFlagSet("config get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	profile := fs.String("printer", "", "profile name")
	if err := fs.Parse(args); err != nil {
		return errExit(err)
	}
	if fs.NArg() < 1 {
		return errExit(errors.New("config get requires a key"))
	}
	key := fs.Arg(0)
	_, cfg, err := loadConfigForEdit(gf)
	if err != nil {
		return errExit(err)
	}
	value := lookupConfigValue(cfg, *profile, key)
	format := selectFormat(gf)
	if format == output.JSON {
		return exitOnErr(output.WriteJSON(os.Stdout, map[string]any{"key": key, "value": value}))
	}
	fmt.Fprintf(os.Stdout, "%v\n", value)
	return 0
}

func cmdConfigSet(gf GlobalFlags, args []string) int {
	fs := flag.NewFlagSet("config set", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	profile := fs.String("printer", "", "profile name")
	ip := fs.String("ip", "", "printer IP")
	serial := fs.String("serial", "", "printer serial")
	accessFile := fs.String("access-code-file", "", "access code file")
	username := fs.String("username", "", "username (default bblp)")
	mqttPort := fs.Int("mqtt-port", 0, "mqtt port")
	ftpPort := fs.Int("ftp-port", 0, "ftp port")
	cameraPort := fs.Int("camera-port", 0, "camera port")
	timeout := fs.Int("timeout", 0, "timeout seconds")
	noCamera := fs.Bool("no-camera", false, "disable camera")
	defaultProfile := fs.Bool("default", false, "set as default profile")
	if err := fs.Parse(args); err != nil {
		return errExit(err)
	}
	if *profile == "" {
		return errExit(errors.New("config set requires --printer"))
	}
	cfgPath, cfg, err := loadConfigForEdit(gf)
	if err != nil {
		return errExit(err)
	}
	p := cfg.Profiles[*profile]
	if *ip != "" {
		p.IP = *ip
	}
	if *serial != "" {
		p.Serial = *serial
	}
	if *accessFile != "" {
		p.AccessCodeFile = *accessFile
	}
	if *username != "" {
		p.Username = *username
	}
	if *mqttPort != 0 {
		p.MQTTPort = *mqttPort
	}
	if *ftpPort != 0 {
		p.FTPPort = *ftpPort
	}
	if *cameraPort != 0 {
		p.CameraPort = *cameraPort
	}
	if *timeout != 0 {
		p.TimeoutSeconds = *timeout
	}
	if *noCamera {
		p.NoCamera = true
	}
	cfg.Profiles[*profile] = p
	if *defaultProfile {
		cfg.DefaultProfile = *profile
	}

	if err := config.Save(cfgPath, cfg); err != nil {
		return errExit(err)
	}
	return 0
}

func cmdConfigRemove(gf GlobalFlags, args []string) int {
	fs := flag.NewFlagSet("config remove", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	profile := fs.String("printer", "", "profile name")
	if err := fs.Parse(args); err != nil {
		return errExit(err)
	}
	if *profile == "" {
		return errExit(errors.New("config remove requires --printer"))
	}
	cfgPath, cfg, err := loadConfigForEdit(gf)
	if err != nil {
		return errExit(err)
	}
	delete(cfg.Profiles, *profile)
	if cfg.DefaultProfile == *profile {
		cfg.DefaultProfile = ""
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		return errExit(err)
	}
	return 0
}

func cmdDoctor(gf GlobalFlags, _ []string) int {
	res, err := resolvePrinter(gf, false, false)
	if err != nil {
		return errExit(err)
	}
	ports := []struct {
		name string
		port int
	}{
		{name: "mqtt", port: res.MQTTPort},
		{name: "ftp", port: res.FTPPort},
		{name: "camera", port: res.CameraPort},
	}
	for _, p := range ports {
		addr := fmt.Sprintf("%s:%d", res.IP, p.port)
		conn, err := net.DialTimeout("tcp", addr, res.Timeout)
		if err != nil {
			fmt.Fprintf(os.Stdout, "%s: failed (%v)\n", p.name, err)
			continue
		}
		_ = conn.Close()
		fmt.Fprintf(os.Stdout, "%s: ok\n", p.name)
	}
	return 0
}

func loadConfigForEdit(gf GlobalFlags) (string, config.Config, error) {
	path, err := config.UserConfigPath()
	if err != nil {
		return "", config.Config{}, err
	}
	if gf.ConfigPath != "" {
		path = gf.ConfigPath
	}
	cfg, err := config.Read(path)
	if err != nil {
		return "", config.Config{}, err
	}
	return path, cfg, nil
}

func lookupConfigValue(cfg config.Config, profileName, key string) any {
	if key == "default_profile" {
		return cfg.DefaultProfile
	}
	if profileName == "" {
		return nil
	}
	p, ok := cfg.Profiles[profileName]
	if !ok {
		return nil
	}
	switch key {
	case "ip":
		return p.IP
	case "serial":
		return p.Serial
	case "access_code_file":
		return p.AccessCodeFile
	case "username":
		return p.Username
	case "mqtt_port":
		return p.MQTTPort
	case "ftp_port":
		return p.FTPPort
	case "camera_port":
		return p.CameraPort
	case "timeout_seconds":
		return p.TimeoutSeconds
	case "no_camera":
		return p.NoCamera
	default:
		return nil
	}
}

func writeAMSPlain(w io.Writer, amsInfo any) error {
	data, err := json.Marshal(amsInfo)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "ams=%s\n", string(data))
	return err
}

func writeAMSHuman(w io.Writer, amsInfo any) error {
	info, ok := amsInfo.(map[string]any)
	if !ok {
		_, err := fmt.Fprintln(w, "No AMS data")
		return err
	}
	if v, ok := info["ams_exist_bits"]; ok {
		if s, ok := v.(string); ok && s == "0" {
			_, err := fmt.Fprintln(w, "No AMS connected")
			return err
		}
	}
	amsList, ok := info["ams"].([]any)
	if !ok || len(amsList) == 0 {
		_, err := fmt.Fprintln(w, "No AMS units found")
		return err
	}
	for _, unit := range amsList {
		m, ok := unit.(map[string]any)
		if !ok {
			continue
		}
		id := fmt.Sprintf("%v", m["id"])
		humidity := fmt.Sprintf("%v", m["humidity"])
		temp := fmt.Sprintf("%v", m["temp"])
		fmt.Fprintf(w, "AMS %s: humidity=%s temp=%s\n", id, humidity, temp)
		trays, _ := m["tray"].([]any)
		for _, tray := range trays {
			tr, ok := tray.(map[string]any)
			if !ok {
				continue
			}
			trayID := fmt.Sprintf("%v", tr["id"])
			name := fmt.Sprintf("%v", tr["tray_id_name"])
			typeName := fmt.Sprintf("%v", tr["tray_type"])
			color := fmt.Sprintf("%v", tr["tray_color"])
			fmt.Fprintf(w, "  tray %s: name=%s type=%s color=%s\n", trayID, name, typeName, color)
		}
	}
	return nil
}

func selectFormat(gf GlobalFlags) output.Format {
	if gf.JSON {
		return output.JSON
	}
	if gf.Plain {
		return output.Plain
	}
	return output.Human
}

func printStatusHuman(status printer.Status) {
	fmt.Fprintf(os.Stdout, "State: %s (%s)\n", status.GcodeState, status.PrintStatus)
	fmt.Fprintf(os.Stdout, "Progress: %d%% (%d/%d)\n", status.Percent, status.LayerCurrent, status.LayerTotal)
	fmt.Fprintf(os.Stdout, "Temps: bed=%sC nozzle=%sC chamber=%sC\n", fmtFloat(status.BedTemp), fmtFloat(status.NozzleTemp), fmtFloat(status.ChamberTemp))
	if status.RemainingMinutes != nil {
		fmt.Fprintf(os.Stdout, "Remaining: %d min\n", *status.RemainingMinutes)
	}
	if status.File != "" {
		fmt.Fprintf(os.Stdout, "File: %s\n", status.File)
	}
	fmt.Fprintf(os.Stdout, "Light: %s\n", status.Light)
	if status.WifiSignal != "" {
		fmt.Fprintf(os.Stdout, "WiFi: %s dBm\n", status.WifiSignal)
	}
	fmt.Fprintf(os.Stdout, "Error: %d\n", status.ErrorCode)
}

func fmtFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64)
}

func formatRemaining(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}

func plateToLocation(plate string) string {
	if plate == "" {
		return "Metadata/plate_1.gcode"
	}
	if _, err := strconv.Atoi(plate); err == nil {
		return fmt.Sprintf("Metadata/plate_%s.gcode", plate)
	}
	return plate
}

func defaultRemoteName(path string) string {
	base := filepath.Base(path)
	lower := strings.ToLower(base)
	if strings.HasSuffix(lower, ".3mf") {
		return base
	}
	if strings.HasSuffix(lower, ".gcode") {
		return strings.TrimSuffix(base, filepath.Ext(base)) + ".3mf"
	}
	return base + ".3mf"
}

func parseIntList(s string) ([]int, error) {
	parts := strings.Split(s, ",")
	out := []int{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		out = []int{0}
	}
	return out, nil
}

func parseFan(s string) (int, error) {
	if strings.Contains(s, ".") {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, err
		}
		if f < 0 || f > 1 {
			return 0, errors.New("fan speed float must be 0-1")
		}
		return int(f * 255), nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if v < 0 || v > 255 {
		return 0, errors.New("fan speed must be 0-255")
	}
	return v, nil
}

func envInt(key string) int {
	v := os.Getenv(key)
	if v == "" {
		return 0
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return i
}

func envBool(key string) bool {
	v := strings.ToLower(os.Getenv(key))
	return v == "1" || v == "true" || v == "yes"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstNonZero(values ...int) int {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}

func errExit(err error) int {
	fmt.Fprintln(os.Stderr, "Error:", err)
	return 1
}

func exitOnErr(err error) int {
	if err != nil {
		return errExit(err)
	}
	return 0
}

func printUsage() {
	fmt.Fprintln(os.Stdout, "bambu-cli - control and monitor BambuLab printers")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "USAGE:")
	fmt.Fprintln(os.Stdout, "  bambu-cli [global flags] <command> [args]")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "COMMANDS:")
	fmt.Fprintln(os.Stdout, "  status                Show printer status")
	fmt.Fprintln(os.Stdout, "  watch                 Watch printer status")
	fmt.Fprintln(os.Stdout, "  light on|off|status    Control printer light")
	fmt.Fprintln(os.Stdout, "  temps get|set          Get or set temperatures")
	fmt.Fprintln(os.Stdout, "  print start|pause|resume|stop")
	fmt.Fprintln(os.Stdout, "  files list|upload|download|delete")
	fmt.Fprintln(os.Stdout, "  camera snapshot        Save camera frame")
	fmt.Fprintln(os.Stdout, "  gcode send             Send gcode line(s)")
	fmt.Fprintln(os.Stdout, "  ams status             Show AMS status")
	fmt.Fprintln(os.Stdout, "  calibrate              Run calibration")
	fmt.Fprintln(os.Stdout, "  home                   Home printer")
	fmt.Fprintln(os.Stdout, "  move z                 Move Z axis")
	fmt.Fprintln(os.Stdout, "  fans set               Set fan speeds")
	fmt.Fprintln(os.Stdout, "  reboot                 Reboot printer")
	fmt.Fprintln(os.Stdout, "  config get|set|list|remove")
	fmt.Fprintln(os.Stdout, "  doctor                 Check connectivity")
	fmt.Fprintln(os.Stdout, "  help [command]         Show help")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "GLOBAL FLAGS:")
	fmt.Fprintln(os.Stdout, "  -h, --help")
	fmt.Fprintln(os.Stdout, "  --version")
	fmt.Fprintln(os.Stdout, "  -q, --quiet")
	fmt.Fprintln(os.Stdout, "  -v, --verbose")
	fmt.Fprintln(os.Stdout, "  --json | --plain")
	fmt.Fprintln(os.Stdout, "  --no-color")
	fmt.Fprintln(os.Stdout, "  --no-input")
	fmt.Fprintln(os.Stdout, "  -f, --force")
	fmt.Fprintln(os.Stdout, "  --confirm <token>")
	fmt.Fprintln(os.Stdout, "  -n, --dry-run")
	fmt.Fprintln(os.Stdout, "  --printer <name>")
	fmt.Fprintln(os.Stdout, "  --ip <addr>")
	fmt.Fprintln(os.Stdout, "  --serial <serial>")
	fmt.Fprintln(os.Stdout, "  --access-code-file <path>")
	fmt.Fprintln(os.Stdout, "  --access-code-stdin")
	fmt.Fprintln(os.Stdout, "  --no-camera")
	fmt.Fprintln(os.Stdout, "  --timeout <seconds>")
	fmt.Fprintln(os.Stdout, "  --config <path>")
}

func printCommandUsage(cmd string) {
	switch cmd {
	case "status":
		fmt.Fprintln(os.Stdout, "USAGE: bambu-cli status")
	case "watch":
		fmt.Fprintln(os.Stdout, "USAGE: bambu-cli watch [--interval <seconds>] [--refresh]")
	case "light":
		fmt.Fprintln(os.Stdout, "USAGE: bambu-cli light on|off|status")
	case "temps":
		fmt.Fprintln(os.Stdout, "USAGE: bambu-cli temps get|set [--bed <C>] [--nozzle <C>] [--chamber <C>]")
	case "print":
		fmt.Fprintln(os.Stdout, "USAGE: bambu-cli print start <file> [--plate <n|path>] [--no-upload]")
		fmt.Fprintln(os.Stdout, "       bambu-cli print pause|resume|stop")
	case "files":
		fmt.Fprintln(os.Stdout, "USAGE: bambu-cli files list [--dir <path>]")
		fmt.Fprintln(os.Stdout, "       bambu-cli files upload <local> [--as <remote>]")
		fmt.Fprintln(os.Stdout, "       bambu-cli files download <remote> --out <path|->")
		fmt.Fprintln(os.Stdout, "       bambu-cli files delete <remote>")
	case "camera":
		fmt.Fprintln(os.Stdout, "USAGE: bambu-cli camera snapshot [--out <path|->]")
	case "gcode":
		fmt.Fprintln(os.Stdout, "USAGE: bambu-cli gcode send <line...> | --stdin")
	case "ams":
		fmt.Fprintln(os.Stdout, "USAGE: bambu-cli ams status")
	case "calibrate":
		fmt.Fprintln(os.Stdout, "USAGE: bambu-cli calibrate [--no-bed-level] [--no-motor-noise] [--no-vibration]")
	case "home":
		fmt.Fprintln(os.Stdout, "USAGE: bambu-cli home")
	case "move":
		fmt.Fprintln(os.Stdout, "USAGE: bambu-cli move z --height <0-256>")
	case "fans":
		fmt.Fprintln(os.Stdout, "USAGE: bambu-cli fans set [--part <0-255|0-1>] [--aux <0-255|0-1>] [--chamber <0-255|0-1>]")
	case "reboot":
		fmt.Fprintln(os.Stdout, "USAGE: bambu-cli reboot")
	case "config":
		fmt.Fprintln(os.Stdout, "USAGE: bambu-cli config list|get|set|remove")
	default:
		printUsage()
	}
}
