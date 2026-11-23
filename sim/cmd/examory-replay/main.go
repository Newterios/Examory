// Command examory-replay replays a simulator world against a local Examory research stack.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func defaultEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func run(ctx context.Context, args []string) (runErr error) {
	fs := flag.NewFlagSet("examory-replay", flag.ContinueOnError)
	mode := fs.String("mode", "parity", "parity or load")
	path := fs.String("world", "", "simulator world CSV")
	keys := fs.String("keys", "", "JSON object: question ID to correct option index (0..3)")
	out := fs.String("out", "", "new private replay manifest JSON")
	rate := fs.Int("rate", 20, "event-bearing operations per second (1..20)")
	duration := fs.Duration("duration", 15*time.Minute, "load window, whole seconds up to 15m")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *path == "" || *keys == "" || *out == "" || (*mode != "load" && *mode != "parity") || *rate < 1 || *rate > 20 || *duration < time.Second || *duration > 15*time.Minute || *duration%time.Second != 0 {
		return errors.New("invalid flags; see --help")
	}
	gw, err := localBase(defaultEnv("E2E_GATEWAY_URL", "http://127.0.0.1:8000"))
	if err != nil {
		return err
	}
	quizzes, err := localBase(defaultEnv("E2E_QUIZZES_URL", "http://127.0.0.1:8001"))
	if err != nil {
		return err
	}
	email, pw := os.Getenv("E2E_ADMIN_EMAIL"), os.Getenv("E2E_ADMIN_PASSWORD")
	if email == "" || pw == "" {
		return errors.New("supply E2E_ADMIN_EMAIL and E2E_ADMIN_PASSWORD through the environment")
	}
	w, err := loadWorld(*path, *keys)
	if err != nil {
		return err
	}
	as := w.actions
	if *mode == "load" {
		as, err = loadPrefix(as, *rate*int(*duration/time.Second))
		if err != nil {
			return err
		}
	}
	id, err := nonce()
	if err != nil {
		return err
	}
	m := &manifest{RunID: id, Mode: *mode, Rate: *rate, WorldDigest: w.digest, KeyDigest: w.keyDigest,
		Courses: map[string]string{}, Users: map[string]string{}, Groups: map[string]string{}, Quizzes: map[string]string{},
		Variants: map[string]string{}, Questions: map[string]questionMapping{}, Attempts: map[string]*attemptMapping{}}
	if *mode == "load" {
		m.Duration = duration.Milliseconds()
	}
	file, err := os.OpenFile(*out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if runErr != nil {
			m.Error = runErr.Error()
		}
		writeErr := json.NewEncoder(file).Encode(m)
		runErr = errors.Join(runErr, writeErr, file.Close())
	}()
	hc := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{Proxy: nil, MaxIdleConnsPerHost: 10}, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	defer hc.CloseIdleConnections()
	api := &api{client: hc, gateway: gw, quizzes: quizzes}
	system, err := api.bootstrap(ctx, w, as, m, email, pw)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "provisioning complete; starting measured replay window")
	return api.execute(ctx, system, w, as, m)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "examory-replay:", err)
		os.Exit(1)
	}
}
