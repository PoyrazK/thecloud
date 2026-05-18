// tar-wrapper provides a safe wrapper around tar for Firecracker snapshots.
// It accepts arguments via stdin JSON to avoid shell injection.
package main

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
)

type tarRequest struct {
	Command     string `json:"cmd"` // "create" or "extract"
	ArchivePath string `json:"archive_path"`
	TargetDir   string `json:"target_dir"`
	FileName    string `json:"file_name"`
}

func main() {
	var req tarRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		log.Fatalf("failed to decode request: %v", err)
	}

	switch req.Command {
	case "create":
		if err := createArchive(req.ArchivePath, req.TargetDir, req.FileName); err != nil {
			log.Fatalf("create failed: %v", err)
		}
	case "extract":
		if err := extractArchive(req.ArchivePath, req.TargetDir); err != nil {
			log.Fatalf("extract failed: %v", err)
		}
	default:
		log.Fatalf("unknown command: %s", req.Command)
	}
}

func createArchive(archivePath, targetDir, fileName string) error {
	cmd := exec.Command("tar", "czf", archivePath, "-C", targetDir, fileName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func extractArchive(archivePath, targetDir string) error {
	cmd := exec.Command("tar", "xzf", archivePath, "-C", targetDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
