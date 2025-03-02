package generate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"

	"github.com/budimanjojo/talhelper/pkg/config"
	"github.com/budimanjojo/talhelper/pkg/patcher"
	"github.com/budimanjojo/talhelper/pkg/talos"
)

// GenerateConfig takes `TalhelperConfig` and path to encrypted `secretFile` and generates
// Talos `machineconfig` files and a `talosconfig` file in `outDir`.
// It returns an error, if any.
func GenerateConfig(c *config.TalhelperConfig, dryRun bool, outDir, secretFile, mode string, offlineMode bool) error {
	var cfg []byte
	input, err := talos.NewClusterInput(c, secretFile)
	if err != nil {
		return err
	}

	for _, node := range c.Nodes {
		fileName := c.ClusterName + "-" + node.Hostname + ".yaml"
		cfgFile := outDir + "/" + fileName

		cfg, err = talos.GenerateNodeConfigBytes(&node, input, c.GetImageFactory(), offlineMode)
		if err != nil {
			return err
		}

		if node.InlinePatch != nil {
			cfg, err = patcher.YAMLInlinePatcher(node.InlinePatch, cfg)
			if err != nil {
				return err
			}
		}

		if len(node.ConfigPatches) != 0 {
			cfg, err = patcher.YAMLPatcher(node.ConfigPatches, cfg)
			if err != nil {
				return err
			}
		}

		if len(node.Patches) != 0 {
			cfg, err = patcher.PatchesPatcher(node.Patches, cfg)
			if err != nil {
				return err
			}
		}

		if node.ControlPlane {
			cfg, err = patcher.YAMLInlinePatcher(c.ControlPlane.InlinePatch, cfg)
			if err != nil {
				return err
			}
			cfg, err = patcher.YAMLPatcher(c.ControlPlane.ConfigPatches, cfg)
			if err != nil {
				return err
			}
			cfg, err = patcher.PatchesPatcher(c.ControlPlane.Patches, cfg)
			if err != nil {
				return err
			}
		} else {
			cfg, err = patcher.YAMLInlinePatcher(c.Worker.InlinePatch, cfg)
			if err != nil {
				return err
			}
			cfg, err = patcher.YAMLPatcher(c.Worker.ConfigPatches, cfg)
			if err != nil {
				return err
			}
			cfg, err = patcher.PatchesPatcher(c.Worker.Patches, cfg)
			if err != nil {
				return err
			}
		}

		if len(c.Patches) > 0 {
			cfg, err = patcher.PatchesPatcher(c.Patches, cfg)
			if err != nil {
				return err
			}
		}

		err = talos.ValidateConfigFromBytes(cfg, mode)
		if err != nil {
			return err
		}

		cfg, err = talos.ReEncodeTalosConfig(cfg)
		if err != nil {
			return err
		}

		if !dryRun {
			err = dumpFile(cfgFile, cfg)
			if err != nil {
				return err
			}

			fmt.Printf("generated config for %s in %s\n", node.Hostname, cfgFile)
		} else {
			absCfgFile, err := filepath.Abs(cfgFile)
			if err != nil {
				return err
			}

			before, err := getFileContent(absCfgFile)
			if err != nil {
				return err
			}

			diff := computeDiff(absCfgFile, before, string(cfg))
			if diff != "" {
				fmt.Println(diff)
			} else {
				fmt.Printf("no changes found on %s\n", cfgFile)
			}
		}
	}

	if !dryRun {
		clientCfg, err := talos.GenerateClientConfigBytes(c, input)
		if err != nil {
			return err
		}

		fileName := "talosconfig"

		err = dumpFile(outDir+"/"+fileName, clientCfg)
		if err != nil {
			return err
		}

		fmt.Printf("generated client config in %s\n", outDir+"/"+fileName)
	}

	return nil
}

// getFileContent returns content of file. It also returns an error,
// if any
func getFileContent(path string) (string, error) {
	if _, osErr := os.Stat(path); osErr == nil {
		content, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(content), nil
	} else if errors.Is(osErr, os.ErrNotExist) {
		return "", nil
	} else {
		return "", osErr
	}
}

// computeDiff returns diff between before and after string
// using Myers diff algorithm
func computeDiff(path, before, after string) string {
	edits := myers.ComputeEdits(span.URIFromPath(path), before, after)
	diff := gotextdiff.ToUnified("a"+path, "b"+path, before, edits)
	return fmt.Sprint(diff)
}

// dumpFile creates file in `path` and dumps the content of bytes into
// the path. It returns an error, if any.
func dumpFile(path string, file []byte) error {
	dirName := filepath.Dir(path)

	_, err := os.Stat(dirName)
	if err != nil {
		err := os.MkdirAll(dirName, 0o700)
		if err != nil {
			return err
		}
	}

	err = os.WriteFile(path, file, 0o600)
	if err != nil {
		return err
	}

	return nil
}
