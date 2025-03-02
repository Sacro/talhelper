package talos

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"text/template"

	"github.com/budimanjojo/talhelper/pkg/config"
	"github.com/siderolabs/image-factory/pkg/schematic"
)

var errNotStatusCreated = errors.New("Server not replying StatusCreated")

type factoryPOSTResult struct {
	ID string `json:"id"`
}

type installerTmpl struct {
	RegistryURL string
	ID          string
	Version     string
}

type isoTmpl struct {
	Protocol    string
	RegistryURL string
	ID          string
	Version     string
	Mode        string
	Arch        string
}

func GetInstallerURL(cfg *schematic.Schematic, factory *config.ImageFactory, version string, offlineMode bool) (string, error) {
	tmplData := installerTmpl{
		RegistryURL: factory.RegistryURL,
		Version:     version,
	}

	id, err := getSchematicID(cfg, factory, offlineMode)
	if err != nil {
		return "", err
	}
	tmplData.ID = id

	t, err := template.New("installer").Parse(factory.InstallerURLTmpl)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	if err := t.Execute(buf, tmplData); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func GetISOURL(cfg *schematic.Schematic, factory *config.ImageFactory, spec *config.MachineSpec, version string, offlineMode bool) (string, error) {
	tmplData := isoTmpl{
		Protocol:    factory.Protocol,
		RegistryURL: factory.RegistryURL,
		Version:     version,
		Mode:        spec.Mode,
		Arch:        spec.Arch,
	}

	id, err := getSchematicID(cfg, factory, offlineMode)
	if err != nil {
		return "", err
	}
	tmplData.ID = id

	t, err := template.New("iso").Parse(factory.ISOURLTmpl)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	if err := t.Execute(buf, tmplData); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func getSchematicID(cfg *schematic.Schematic, iFactory *config.ImageFactory, offlineMode bool) (string, error) {
	if offlineMode {
		id, err := cfg.ID()
		if err != nil {
			return "", err
		}
		return id, nil
	}
	body, err := cfg.Marshal()
	if err != nil {
		return "", err
	}
	var resp factoryPOSTResult
	schematicURL := iFactory.Protocol + "://" + iFactory.RegistryURL + iFactory.SchematicEndpoint
	if err := doHTTPPOSTRequest(body, schematicURL, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

func doHTTPPOSTRequest(body []byte, url string, out interface{}) error {
	resp, err := http.Post(url, "", bytes.NewReader(body))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("%w (%v): %v", errNotStatusCreated, url, resp.Status)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	return nil
}
