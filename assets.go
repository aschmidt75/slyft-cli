package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	cli "github.com/jawher/mow.cli"
	"github.com/olekukonko/tablewriter"
)

type Asset struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	ProjectId   int       `json:"project_id"`
	ProjectName string    `json:"project_name"`
	Origin      string    `json:"origin"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (a *Asset) getName() string {
	return a.Name
}

func (a *Asset) Display() { // String?
	if a == nil {
		return
	}

	var data [][]string
	data = append(data, []string{"Name", a.Name})
	data = append(data, []string{"ProjectId", fmt.Sprintf("%d", a.ProjectId)})
	data = append(data, []string{"ProjectName", a.ProjectName})
	data = append(data, []string{"Origin", a.Origin})
	data = append(data, []string{"CreatedAt", a.CreatedAt.String()})
	data = append(data, []string{"UpdatedAt", a.UpdatedAt.String()})

	table := tablewriter.NewWriter(os.Stdout)
	table.SetColWidth(TerminalWidth())
	table.SetHeader([]string{"Key", "Value"})
	table.SetBorder(false)
	table.AppendBulk(data)
	fmt.Fprintf(os.Stdout, "\n---- Asset Details ----\n")
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.Render()
	fmt.Fprintf(os.Stdout, "\n")
}

func DisplayAssets(assets []Asset) {
	if len(assets) == 0 {
		fmt.Println("No assets found")
		return
	}

	if len(assets) == 1 {
		assets[0].Display()
		return
	}

	var data [][]string
	for i := range assets {
		a := assets[i]
		data = append(data, []string{fmt.Sprintf("%d", i+1), a.Name, a.ProjectName, a.Origin})
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetColWidth(TerminalWidth())
	table.SetHeader([]string{"Number", "Name", "Project Name", "Origin"})
	table.SetBorder(false)
	table.AppendBulk(data)
	fmt.Fprintf(os.Stdout, "\n")
	table.Render()
	fmt.Fprintf(os.Stdout, "\n")
}

func extractAssetsFromBody(body []byte) ([]Asset, error) {
	Log.Debugf("body=%v", string(body))
	assets := make([]Asset, 0)
	if err := json.Unmarshal(body, &assets); err != nil {
		return nil, err
	}
	return assets, nil
}

func extractAssetFromBody(body []byte) ([]Asset, error) {
	Log.Debugf("body=%v", string(body))
	a := &Asset{}
	if err := json.Unmarshal(body, &a); err != nil {
		return nil, err
	}
	return []Asset{*a}, nil
}

func chooseAsset(endpoint string, askUser bool, message string, count int) (*Asset, error) {
	resp, err := Do(endpoint, "GET", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	assets, err := extractAssetFromResponse(resp, http.StatusOK, true)

	if err != nil {
		return nil, err
	}

	if len(assets) == 0 {
		return nil, errors.New("No asset. Sorry")
	}
	Log.Debugf("assets=%+v", assets)

	if count != 0 {
		assets = assets[len(assets)-count:]
	}

	DisplayAssets(assets)

	if len(assets) == 1 {
		return &assets[0], nil
	}

	if !askUser {
		return nil, nil
	}

	choice, err := ReadUserIntInput(message)
	if err != nil {
		return nil, err
	}

	if choice > len(assets) {
		return nil, errors.New("Plese choose a number from the first column")
	}

	return &assets[choice-1], nil
}

type AssetPost struct {
	Name  string `json:"name"`
	Asset string `json:"asset"` // note: this will be base64 string
}

type AssetParam struct {
	Asset AssetPost `json:"asset"`
}

type AssetNameString struct {
	AssetNameString string `json:"asset_name"`
}

func creatAssetParam(file string) (*AssetParam, error) {
	// read the file content (use ioutil)
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	mimeType, err := preflightAsset(&bytes, file)
	if err != nil {
		return nil, err
	}

	return &AssetParam{
		AssetPost{
			Name:  file,
			Asset: "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(bytes),
		},
	}, nil
}

func readFileAndPostAsset(file string, p *Project) {
	assetParam, err := creatAssetParam(file)
	if err != nil {
		ReportError("Creating request", err)
		return
	}

	resp, err := Do(p.AssetsUrl(), "POST", assetParam)
	if err != nil {
		ReportError("Contacting server", err)
		return
	}
	defer resp.Body.Close()
	assets, err := extractAssetFromResponse(resp, http.StatusCreated, false)
	if err != nil {
		ReportError("Creating asset", err)
		return
	}

	if len(assets) == 1 {
		assets[0].Display()
	}
}

func getAssetAndSaveToFile(file string, p *Project) {
	resp, err := Do(p.AssetstoreUrl(), "GET", &AssetNameString{file})
	if err != nil {
		ReportError("Downloading asset", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		// stream body to file of this name
		out, err := os.Create(file)
		if err != nil {
			ReportError("Creating asset file", err)
			return
		}
		defer out.Close()
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			ReportError("Writing asset file", err)
			return
		}
		fmt.Printf("Downloaded %s\n", file)
	} else {
		ReportError("Downloading asset", nil)
	}
}

func listAssets(cmd *cli.Cmd) {
	cmd.Spec = "[--project] | [--all]"
	name := cmd.StringOpt("project p", "", "Name (or part of it) of a project")
	all := cmd.BoolOpt("all a", false, "Fetch details of all your assets (do not combine with -p)")

	cmd.Action = func() {
		*name = strings.TrimSpace(*name)
		if *all {
			chooseAsset("/v1/assets", false, "", 0)
			return
		} else {
			if *name == "" {
				*name, _ = ReadProjectLock()
			}
		}

		// first get the project, then get the pid, and make the call.
		p, err := chooseProject(*name, "Which project's assets would you like to see: ")
		if err != nil {
			ReportError("Choosing the project", err)
			return
		}
		if _, err = chooseAsset(p.AssetsUrl(), false, "", 0); err != nil {
			ReportError("Choosing the asset", err)
		}
	}
}

func addAsset(cmd *cli.Cmd) {
	cmd.Spec = "[--project] --file"
	name := cmd.StringOpt("project p", "", "Name (or part of it) of a project")
	file := cmd.StringOpt("file f", "", "path to the file which you want as an asset")

	cmd.Action = func() {
		*name = strings.TrimSpace(*name)

		if *name == "" {
			*name, _ = ReadProjectLock()
		}

		// first get the project, then get the pid, and make the call.
		p, err := chooseProject(*name, "Add asset to: ")
		if err != nil {
			ReportError("Choosing the project", err)
			return
		}

		*file = strings.TrimSpace(*file)
		readFileAndPostAsset(*file, p)
	}
}

func getAsset(cmd *cli.Cmd) {
	cmd.Spec = "[--project] --file"
	name := cmd.StringOpt("project p", "", "Name (or part of it) of a project")
	file := cmd.StringOpt("file f", "", "name of the asset to be downloaded")

	cmd.Action = func() {
		*name = strings.TrimSpace(*name)

		if *name == "" {
			*name, _ = ReadProjectLock()
		}
		// first get the project, then get the pid, and make the call.
		p, err := chooseProject(*name, "Download asset from: ")
		if err != nil {
			ReportError("Choosing the project", err)
			return
		}

		*file = strings.TrimSpace(*file)
		getAssetAndSaveToFile(*file, p)
	}
}

func (ass *Asset) EndPoint() string {
	return fmt.Sprintf("/v1/projects/%d/assets/%d", ass.ProjectId, ass.ID)
}

func removeAsset(cmd *cli.Cmd) {
	cmd.Spec = "[--project] [--all | --count]"
	name := cmd.StringOpt("project p", "", "Name (or part of it) of a project")
	all := cmd.BoolOpt("all a", false, "Fetch details of all your assets (do not combine with -n)")
	count := cmd.IntOpt("count n", 1, "Delete the last 'count' asssets of the project (do not compine with -a)")

	cmd.Action = func() {
		*name = strings.TrimSpace(*name)
		if *name == "" {
			*name, _ = ReadProjectLock()
		}

		var ass *Asset
		var err error
		if *all {
			ass, err = chooseAsset("/v1/assets", true, "Which one shall be deleted: ", 0)
		} else if *name == "" {
			ass, err = chooseAsset("/v1/assets", true, "Which one shall be deleted: ", *count)
		} else {
			// first get the project, then get the pid, and make the call.
			p, err2 := chooseProject(*name, "Which project's assets would you like to see: ")
			if err2 != nil {
				ReportError("Choosing the project", err)
				return
			}
			ass, err = chooseAsset(p.AssetsUrl(), true, "Which one shall be removed: ", *count)
		}

		if err != nil {
			ReportError("Choosing the asset", err)
			return
		}

		DeleteApiModel(ass)
	}
}

func RegisterAssetRoutes(proj *cli.Cmd) {
	SetupLogger()

	proj.Command("add a", "Add asset to a project", addAsset)
	proj.Command("list ls", "List your assets", listAssets)
	proj.Command("get g", "Download a single asset", getAsset)
	proj.Command("delete d", "Remove and asset from a project", removeAsset)
}
