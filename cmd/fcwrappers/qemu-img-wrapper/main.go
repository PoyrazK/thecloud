// qemu-img-wrapper provides a safe wrapper around qemu-img for Firecracker snapshots.
// It accepts arguments via stdin JSON to avoid shell injection.
package main

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
)

type qemuImgRequest struct {
	Command    string `json:"cmd"` // "convert"
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path"`
	Format     string `json:"format"` // "qcow2", "raw", etc.
}

func main() {
	var req qemuImgRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		log.Fatalf("failed to decode request: %v", err)
	}

	if err := convertImage(req.SourcePath, req.TargetPath, req.Format); err != nil {
		log.Fatalf("convert failed: %v", err)
	}
}

func convertImage(sourcePath, targetPath, format string) error {
	args := []string{"convert", "-O", format, sourcePath, targetPath}
	cmd := exec.Command("qemu-img", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
