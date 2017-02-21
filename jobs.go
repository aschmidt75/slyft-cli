package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	cli "github.com/jawher/mow.cli"
	"github.com/olekukonko/tablewriter"
)

type Job struct {
	ID          int        `json:"id"`
	Kind        string     `json:"kind"`
	Status      string     `json:"status"`
	Results     JobResults `json:"results"`
	ProjectId   int        `json:"project_id"`
	ProjectName string     `json:"project_name"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type JobResults struct {
	ResultMessage string   `json:"resultMessage"`
	ResultStatus  int      `json:"resultStatus"`
	ResultAssets  []string `json:"resultAssets"`
	ResultDetails []string `json:"resultDetails"`
}

func (j *Job) Display() { // String?
	if j == nil {
		return
	}

	var data [][]string
	data = append(data, []string{"Id", fmt.Sprintf("%d", j.ID)})
	data = append(data, []string{"Kind", j.Kind})
	data = append(data, []string{"Status", j.Status})
	data = append(data, []string{"ProjectId", fmt.Sprintf("%d", j.ProjectId)})
	data = append(data, []string{"ProjectName", j.ProjectName})
	data = append(data, []string{"CreatedAt", j.CreatedAt.String()})
	data = append(data, []string{"UpdatedAt", j.UpdatedAt.String()})
	data = append(data, []string{"ResultMessage", j.Results.ResultMessage})
	data = append(data, []string{"ResultStatus", fmt.Sprintf("%d", j.Results.ResultStatus)})
	for index, asset := range j.Results.ResultAssets {
		data = append(data, []string{fmt.Sprintf("ResultAssets[%d]", index), asset})
	}
	// Details are an array.
	for index, detail := range j.Results.ResultDetails {
		data = append(data, []string{fmt.Sprintf("ResultDetails[%d]", index), detail})
	}
	//data = append(data, []string{"ResultDetails", fmt.Sprintf("%s", j.Results.ResultDetails)})

	table := tablewriter.NewWriter(os.Stdout)
	table.SetColWidth(TerminalWidth())
	table.SetHeader([]string{"Key", "Value"})
	table.SetBorder(false)
	table.AppendBulk(data)
	fmt.Fprintf(os.Stdout, "\n---- Job Details ----\n")
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.Render()
	fmt.Fprintf(os.Stdout, "\n")
}

func DisplayJobs(jobs []Job) {
	if len(jobs) == 0 {
		fmt.Println("No jobs found")
		return
	}

	if len(jobs) == 1 {
		jobs[0].Display()
		return
	}

	var data [][]string
	for i := range jobs {
		j := jobs[i]
		data = append(data, []string{fmt.Sprintf("%d", i+1), fmt.Sprintf("%d", j.ID), j.Kind, j.Status, j.ProjectName})
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetColWidth(TerminalWidth())
	table.SetHeader([]string{"Number", "ID", "Kind", "Status", "Project Name"})
	table.SetBorder(false)
	table.AppendBulk(data)
	fmt.Fprintf(os.Stdout, "\n")
	table.Render()
	fmt.Fprintf(os.Stdout, "\n")
}

func extractJobsFromBody(body []byte) ([]Job, error) {
	jobs := make([]Job, 0)
	if err := json.Unmarshal(body, &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

func extractJobFromBody(body []byte) ([]Job, error) {
	a := &Job{}
	if err := json.Unmarshal(body, &a); err != nil {
		return nil, err
	}
	return []Job{*a}, nil
}

func chooseJob(endpoint string, askUser bool, message string) (*Job, error) {
	resp, err := Do(endpoint, "GET", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	jobs, err := extractJobFromResponse(resp, http.StatusOK, true)

	if err != nil {
		return nil, err
	}

	if len(jobs) == 0 {
		return nil, errors.New("No job. Sorry")
	}

	DisplayJobs(jobs)

	if len(jobs) == 1 {
		return &jobs[0], nil
	}

	if !askUser {
		return nil, nil
	}

	choice, err := ReadUserIntInput(message)
	if err != nil {
		return nil, err
	}

	if choice > len(jobs) {
		return nil, errors.New("Plese choose a number from the first column")
	}

	return &jobs[choice-1], nil
}

type JobParam struct {
	Job Job `json:"job"`
}

func creatJobParam(kind string, p *Project) *JobParam {
	return &JobParam{
		Job{
			Kind:      kind,
			ProjectId: p.ID,
		},
	}
}

func postNewJob(kind, name string) *Job {
	p, err := chooseProject(name, fmt.Sprintf("%s project: ", kind))
	if err != nil {
		ReportError("Choosing a project", err)
		return nil
	}

	resp, err := Do(p.JobsUrl(), "POST", creatJobParam(kind, p))
	if err != nil {
		ReportError("Contacting the server", err)
		return nil
	}

	defer resp.Body.Close()
	jobs, err := extractJobFromResponse(resp, http.StatusCreated, false)
	if err != nil {
		ReportError("Reading the server response", err)
		return nil
	}

	Log.Debugf("jobs=%#v", jobs)
	if len(jobs) == 1 {
		jobs[0].Display()
		return &(jobs[0])
	} else {
		Log.Errorf("Error, creating a new job returned wrong job")
	}
	return nil
}

func jobStatusProject(cmd *cli.Cmd) {
	cmd.Spec = "[--project] | [--all]"
	name := cmd.StringOpt("project p", "", "Name (or part of it) of a project")
	all := cmd.BoolOpt("all a", false, "Fetch details of all your jobs (do not combine with -p)")

	cmd.Action = func() {
		*name = strings.TrimSpace(*name)
		if *name == "" {
			*name, _ = ReadProjectLock()
		}
		if *all || *name == "" {
			_, err := chooseJob("/v1/jobs", false, "")
			ReportError("Choosing the job", err)
		}

		// first get the project, then get the pid, and make the call.
		p, err := chooseProject(*name, "Which project's jobs would you like to see: ")
		if err != nil {
			ReportError("Choosing the project", err)
			return
		}
		job, err := chooseJob(p.JobsUrl(), true, "Select a job id to show more details: ")
		if err != nil {
			ReportError("Selecting the job", err)
			return
		}
		job.Display()
	}
}

func waitForJobCompletion(job *Job, wait int) bool {
	fmt.Printf("Waiting (%ds) for job completion..", wait)
	for wait > 0 {
		wait -= 5
		time.Sleep(5 * time.Second)
		fmt.Print(".")

		resp, err := Do(job.EndPoint(), "GET", nil)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		jobs, err := extractJobFromResponse(resp, http.StatusOK, false)
		Log.Debugf("jobs=%#v", jobs)
		//Log.Debugf("#jobs=%d", len(jobs))
		//Log.Debugf("err=%#v", err)

		if err == nil && jobs != nil && len(jobs) == 1 {
			Log.Debugf("status=%s", jobs[0].Status)
			if jobs[0].Status == "processed" {
				jobs[0].Display();
				return true
			}
		}

	}
	// if we get here, job did not finish in time. Say so.
	fmt.Printf("Job %d did not complete in time. Please check manually using `slyft project status`", job.ID)
	return false
}

func buildProject(cmd *cli.Cmd) {
	cmd.Spec = "[--project] [--wait]"
	name := cmd.StringOpt("project p", "", "Name (or part of it) of a project")
	wait := cmd.IntOpt("wait w", 30, "Optional number of seconds to wait for job completion")
	if *name == "" {
		*name, _ = ReadProjectLock()
	}

	cmd.Action = func() {
		job := postNewJob("build", strings.TrimSpace(*name))
		if wait != nil {
			waitForJobCompletion(job, *wait)
		}
	}
}

func validateProject(cmd *cli.Cmd) {
	cmd.Spec = "[--project] [--wait]"
	name := cmd.StringOpt("project p", "", "Name (or part of it) of a project")
	wait := cmd.IntOpt("wait w", 30, "Optional number of seconds to wait for job completion")
	if *name == "" {
		*name, _ = ReadProjectLock()
	}

	cmd.Action = func() {
		job := postNewJob("validate", strings.TrimSpace(*name))
		if wait != nil {
			waitForJobCompletion(job, *wait)
		}
	}
}

func (job *Job) EndPoint() string {
	return fmt.Sprintf("/v1/projects/%d/jobs/%d", job.ProjectId, job.ID)
}
