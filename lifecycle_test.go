package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCanonicalStateDir(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("canonicalStateDir resolves via XDG_CONFIG_HOME on linux only")
	}
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	dir, err := canonicalStateDir()
	if err != nil {
		t.Fatalf("canonicalStateDir: %v", err)
	}
	if want := filepath.Join(tmp, "herdr-whistle"); dir != want {
		t.Errorf("dir = %q, want %q", dir, want)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Errorf("state dir not created at %s: %v", dir, err)
	}
}

func TestAcquireInstanceLockExclusive(t *testing.T) {
	dir := t.TempDir()

	first, err := acquireInstanceLock(dir)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	t.Cleanup(func() { first.Close() })

	if _, err := acquireInstanceLock(dir); !errors.Is(err, errLockHeld) {
		t.Fatalf("second acquire err = %v, want errLockHeld", err)
	}

	// Releasing lets a fresh acquire succeed.
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	second, err := acquireInstanceLock(dir)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	t.Cleanup(func() { second.Close() })
}

func TestInstanceLocked(t *testing.T) {
	dir := t.TempDir()

	held, err := instanceLocked(dir)
	if err != nil {
		t.Fatalf("instanceLocked free: %v", err)
	}
	if held {
		t.Fatal("reported locked while free")
	}

	lock, err := acquireInstanceLock(dir)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	t.Cleanup(func() { lock.Close() })

	held, err = instanceLocked(dir)
	if err != nil {
		t.Fatalf("instanceLocked held: %v", err)
	}
	if !held {
		t.Fatal("reported free while locked")
	}
}

func TestPidFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := writePidFile(dir, 4242); err != nil {
		t.Fatalf("writePidFile: %v", err)
	}
	got, err := readPidFile(dir)
	if err != nil {
		t.Fatalf("readPidFile: %v", err)
	}
	if got != 4242 {
		t.Errorf("pid = %d, want 4242", got)
	}
}

func TestReadPidFileMissing(t *testing.T) {
	if _, err := readPidFile(t.TempDir()); err == nil {
		t.Fatal("expected error for missing pidfile")
	}
}

func TestProcessAlive(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Fatal("processAlive(self) = false")
	}
	if !processAlive(1) {
		t.Fatal("processAlive(init) = false")
	}
	if processAlive(2_000_000) {
		t.Fatal("processAlive(unused pid) = true")
	}
	if processAlive(0) || processAlive(-1) {
		t.Fatal("processAlive(non-positive) = true")
	}
}
