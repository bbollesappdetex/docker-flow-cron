package server

import (
	"../cron"
	"../docker"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types/swarm"
	"github.com/stretchr/testify/suite"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"
)

type ServerTestSuite struct {
	suite.Suite
	ResponseWriter ResponseWriterMock
	Service        ServicerMock
}

func (s *ServerTestSuite) SetupTest() {
	s.ResponseWriter = ResponseWriterMock{
		WriteHeaderMock: func(header int) {
		},
		HeaderMock: func() http.Header {
			return http.Header{}
		},
		WriteMock: func(content []byte) (int, error) {
			return 0, nil
		},
	}
	s.Service = ServicerMock{
		GetServicesMock: func(jobName string) ([]swarm.Service, error) {
			return []swarm.Service{}, nil
		},
		GetTasksMock: func(jobName string) ([]swarm.Task, error) {
			return []swarm.Task{}, nil
		},
	}
}

func TestCronUnitTestSuite(t *testing.T) {
	s := new(ServerTestSuite)
	suite.Run(t, s)
}

// New

func (s *ServerTestSuite) Test_New_ReturnsError_WhenDockerClientFails() {
	_, err := New("myIp", "1234", "this-is-not-a-socket")

	s.Error(err)
}

// Execute

func (s *ServerTestSuite) Test_Execute_InvokesHTTPListenAndServe() {
	serve, _ := New("myIp", "1234", "unix:///var/run/docker.sock")
	var actual string
	expected := fmt.Sprintf("%s:%s", serve.IP, serve.Port)
	httpListenAndServe = func(addr string, handler http.Handler) error {
		actual = addr
		return nil
	}

	serve.Execute()
	time.Sleep(1 * time.Millisecond)

	s.Equal(expected, actual)
}

func (s *ServerTestSuite) Test_Execute_ReturnsError_WhenHTTPListenAndServeFails() {
	orig := httpListenAndServe
	defer func() { httpListenAndServe = orig }()
	httpListenAndServe = func(addr string, handler http.Handler) error {
		return fmt.Errorf("This is an error")
	}

	serve, _ := New("myIp", "1234", "unix:///var/run/docker.sock")
	actual := serve.Execute()

	s.Error(actual)
}

// JobPutHandler

func (s *ServerTestSuite) Test_JobPutHandler_InvokesCronAddJob() {
	muxVarsOrig := muxVars
	defer func() { muxVars = muxVarsOrig }()
	muxVars = func(r *http.Request) map[string]string {
		return map[string]string{"jobName": "my-job"}
	}
	expectedData := cron.JobData{
		Image:    "my-image",
		Schedule: "@yearly",
	}
	actualData := cron.JobData{}
	js, _ := json.Marshal(expectedData)
	expectedData.Name = "my-job"
	req, _ := http.NewRequest(
		"PUT",
		"/v1/docker-flow-cron/job",
		strings.NewReader(string(js)),
	)
	cMock := CronerMock{
		AddJobMock: func(data cron.JobData) error {
			actualData = data
			return nil
		},
	}

	srv := Serve{Cron: cMock}
	srv.JobPutHandler(s.ResponseWriter, req)

	s.Equal(expectedData, actualData)
}

func (s *ServerTestSuite) Test_JobPutHandler_ReturnsBadRequestWhenBodyIsNil() {
	req, _ := http.NewRequest("PUT", "/v1/docker-flow-cron/job", nil)
	cMock := CronerMock{
		AddJobMock: func(data cron.JobData) error {
			return nil
		},
	}
	actual := 0
	mock := ResponseWriterMock{
		WriteHeaderMock: func(header int) {
			actual = header
		},
		HeaderMock: func() http.Header {
			return http.Header{}
		},
		WriteMock: func(content []byte) (int, error) {
			return 0, nil
		},
	}

	srv := Serve{Cron: cMock}
	srv.JobPutHandler(mock, req)

	s.Equal(400, actual)
}

func (s *ServerTestSuite) Test_JobPutHandler_InvokesInternalServerError_WhenAddJobFails() {
	expectedData := cron.JobData{
		Name:     "my-job",
		Image:    "my-image",
		Schedule: "@yearly",
	}
	js, _ := json.Marshal(expectedData)
	req, _ := http.NewRequest(
		"PUT",
		"/v1/docker-flow-cron/job",
		strings.NewReader(string(js)),
	)
	cMock := CronerMock{
		AddJobMock: func(data cron.JobData) error {
			return fmt.Errorf("This is an error")
		},
	}
	actualStatus := 0
	mock := ResponseWriterMock{
		WriteHeaderMock: func(header int) {
			actualStatus = header
		},
		HeaderMock: func() http.Header {
			return http.Header{}
		},
		WriteMock: func(content []byte) (int, error) {
			return 0, nil
		},
	}

	srv := Serve{Cron: cMock}
	srv.JobPutHandler(mock, req)

	s.Equal(500, actualStatus)
}

// JobGetHandler

func (s *ServerTestSuite) Test_JobGetHandler_ReturnsListOfServices() {
	jobs := map[string]cron.JobData{
		"my-job-1": {},
		"my-job-2": {},
	}
	req, _ := http.NewRequest("GET", "/v1/docker-flow-cron/job", nil)
	expected := Response{
		Status: "OK",
		Jobs:   jobs,
	}
	actual := Response{}
	rwMock := ResponseWriterMock{
		WriteHeaderMock: func(header int) {},
		HeaderMock: func() http.Header {
			return http.Header{}
		},
		WriteMock: func(content []byte) (int, error) {
			json.Unmarshal(content, &actual)
			return 0, nil
		},
	}
	cMock := CronerMock{
		GetJobsMock: func() (map[string]cron.JobData, error) {
			return jobs, nil
		},
	}

	srv := Serve{Service: s.Service, Cron: cMock}
	srv.JobGetHandler(rwMock, req)

	s.Equal(expected, actual)
}

func (s *ServerTestSuite) Test_JobGetHandler_ReturnsError_WhenGetJobsFail() {
	message := "This is an error"
	actual := Response{}
	actualStatus := 0
	rwMock := ResponseWriterMock{
		WriteHeaderMock: func(header int) {
			actualStatus = header
		},
		HeaderMock: func() http.Header {
			return http.Header{}
		},
		WriteMock: func(content []byte) (int, error) {
			json.Unmarshal(content, &actual)
			return 0, nil
		},
	}
	cMock := CronerMock{
		GetJobsMock: func() (map[string]cron.JobData, error) {
			return map[string]cron.JobData{}, fmt.Errorf("This is an error")
		},
	}
	req, _ := http.NewRequest("GET", "/v1/docker-flow-cron/job", nil)
	expected := Response{
		Status:  "NOK",
		Message: message,
		Jobs:    map[string]cron.JobData{},
	}

	srv := Serve{Service: s.Service, Cron: cMock}
	srv.JobGetHandler(rwMock, req)

	s.Equal(expected, actual)
	s.Equal(500, actualStatus)
}

// JobDeleteHandler

func (s *ServerTestSuite) Test_JobDeleteHandler_ReturnsJobDetails() {
	muxVarsOrig := muxVars
	defer func() { muxVars = muxVarsOrig }()
	muxVars = func(r *http.Request) map[string]string {
		return map[string]string{"jobName": "my-job"}
	}
	name := "my-job"
	req, _ := http.NewRequest(
		"DELETE",
		fmt.Sprintf("/v1/docker-flow-cron/job/%s", name),
		nil,
	)
	expected := ResponseDetails{
		Status:  "OK",
		Message: "my-job was deleted",
	}
	actual := ResponseDetails{}
	rwMock := ResponseWriterMock{
		WriteHeaderMock: func(header int) {},
		HeaderMock: func() http.Header {
			return http.Header{}
		},
		WriteMock: func(content []byte) (int, error) {
			json.Unmarshal(content, &actual)
			return 0, nil
		},
	}
	actualName := ""
	cMock := CronerMock{
		RemoveJobMock: func(jobName string) error {
			actualName = jobName
			return nil
		},
	}

	srv := Serve{Service: s.Service, Cron: cMock}
	srv.JobDeleteHandler(rwMock, req)

	s.Equal(expected, actual)
	s.Equal("my-job", actualName)
}

func (s *ServerTestSuite) Test_JobDeleteHandler_ReturnsNok_WhenRemoveJobFails() {
	muxVarsOrig := muxVars
	defer func() { muxVars = muxVarsOrig }()
	muxVars = func(r *http.Request) map[string]string {
		return map[string]string{"jobName": "my-job"}
	}
	name := "my-job"
	req, _ := http.NewRequest(
		"DELETE",
		fmt.Sprintf("/v1/docker-flow-cron/job/%s", name),
		nil,
	)
	expected := ResponseDetails{
		Status:  "NOK",
		Message: "This is an error",
	}
	actual := ResponseDetails{}
	actualStatus := 0
	rwMock := ResponseWriterMock{
		WriteHeaderMock: func(header int) {
			actualStatus = header
		},
		HeaderMock: func() http.Header {
			return http.Header{}
		},
		WriteMock: func(content []byte) (int, error) {
			json.Unmarshal(content, &actual)
			return 0, nil
		},
	}
	cMock := CronerMock{
		RemoveJobMock: func(jobName string) error {
			return fmt.Errorf("This is an error")
		},
	}

	srv := Serve{Service: s.Service, Cron: cMock}
	srv.JobDeleteHandler(rwMock, req)

	s.Equal(expected, actual)
	s.Equal(500, actualStatus)
}

// JobDetailsHandler

func (s *ServerTestSuite) Test_JobDetailsHandler_ReturnsJobDetails() {
	muxVarsOrig := muxVars
	defer func() { muxVars = muxVarsOrig }()
	muxVars = func(r *http.Request) map[string]string {
		return map[string]string{"jobName": "my-job"}
	}
	defer exec.Command("/bin/sh", "-c", `docker service rm $(docker service ls)`).CombinedOutput()
	name := "my-job"
	req, _ := http.NewRequest(
		"GET",
		fmt.Sprintf("/v1/docker-flow-cron/job/%s", name),
		nil,
	)
	image := "alpine:3.5@sha256:dfbd4a3a8ebca874ebd2474f044a0b33600d4523d03b0df76e5c5986cb02d7e8"
	executions := []Execution{}
	cmdf := `docker service create \
    -l 'com.df.cron=true' \
    -l 'com.df.cron.name=%s' \
    -l 'com.df.cron.schedule=@every 1s' \
    -l 'com.df.cron.command=docker service create --restart-condition none alpine echo "Hello World!"' \
    --constraint "node.labels.env != does-not-exist" \
    --container-label 'container=label' \
    --name %s \
    --restart-condition none %s \
    echo "Hello world!"`
	for _, jobName := range []string{"my-job", "my-job", "some-other-job"} {
		cmd := fmt.Sprintf(
			cmdf,
			jobName,
			jobName,
			image,
		)
		exec.Command("/bin/sh", "-c", cmd).CombinedOutput()
		if jobName == "my-job" {
			executions = append(executions, Execution{})
		}
	}
	job := cron.JobData{
		Name:     name,
		Image:    image,
		ServiceName: name,
		Command:  `docker service create --restart-condition none alpine echo "Hello World!"`,
		Schedule: "@every 1s",
	}
	expected := ResponseDetails{
		Status:     "OK",
		Job:        job,
		Executions: executions,
	}
	actual := ResponseDetails{}
	rwMock := ResponseWriterMock{
		WriteHeaderMock: func(header int) {},
		HeaderMock: func() http.Header {
			return http.Header{}
		},
		WriteMock: func(content []byte) (int, error) {
			json.Unmarshal(content, &actual)
			return 0, nil
		},
	}
	service, _ := docker.New("unix:///var/run/docker.sock")

	srv := Serve{Service: service}
	srv.JobDetailsHandler(rwMock, req)

	s.Equal(expected.Job, actual.Job)
	s.Equal(1, len(actual.Executions))
	s.False(actual.Executions[0].CreatedAt.IsZero())
	s.NotNil(actual.Executions[0].Status)
	s.NotNil(actual.Executions[0].ServiceId)
}

func (s *ServerTestSuite) Test_JobDetailsHandler_ReturnsError_WhenGetServicesFail() {
	message := "This is an get services error"
	mock := ServicerMock{
		GetServicesMock: func(jobName string) ([]swarm.Service, error) {
			return []swarm.Service{}, fmt.Errorf(message)
		},
		GetTasksMock: func(jobName string) ([]swarm.Task, error) {
			return []swarm.Task{}, nil
		},
	}
	actual := ResponseDetails{}
	actualStatus := 0
	rwMock := ResponseWriterMock{
		WriteHeaderMock: func(header int) {
			actualStatus = header
		},
		HeaderMock: func() http.Header {
			return http.Header{}
		},
		WriteMock: func(content []byte) (int, error) {
			json.Unmarshal(content, &actual)
			return 0, nil
		},
	}
	req, _ := http.NewRequest("GET", "/v1/docker-flow-cron/job/my-job", nil)
	expected := ResponseDetails{
		Status:     "NOK",
		Message:    message,
		Job:        cron.JobData{},
		Executions: []Execution{},
	}

	srv := Serve{Service: mock}
	srv.JobDetailsHandler(rwMock, req)

	s.Equal(expected, actual)
	s.Equal(500, actualStatus)
}

func (s *ServerTestSuite) Test_JobDetailsHandler_ReturnsError_WhenServiceDoesNotExist() {
	actual := ResponseDetails{}
	actualStatus := 0
	rwMock := ResponseWriterMock{
		WriteHeaderMock: func(header int) {
			actualStatus = header
		},
		HeaderMock: func() http.Header {
			return http.Header{}
		},
		WriteMock: func(content []byte) (int, error) {
			json.Unmarshal(content, &actual)
			return 0, nil
		},
	}
	req, _ := http.NewRequest("GET", "/v1/docker-flow-cron/job/my-job", nil)
	expected := ResponseDetails{
		Status:     "NOK",
		Message:    "Could not find the job",
		Job:        cron.JobData{},
		Executions: []Execution{},
	}

	srv := Serve{Service: s.Service}
	srv.JobDetailsHandler(rwMock, req)

	s.Equal(expected, actual)
	s.Equal(404, actualStatus)
}

func (s *ServerTestSuite) Test_JobDetailsHandler_ReturnsError_WhenGetTasksFail() {
	message := "This is an get tasks error"
	mock := ServicerMock{
		GetServicesMock: func(jobName string) ([]swarm.Service, error) {
			return []swarm.Service{{}}, nil
		},
		GetTasksMock: func(jobName string) ([]swarm.Task, error) {
			return []swarm.Task{}, fmt.Errorf(message)
		},
	}
	actual := ResponseDetails{}
	actualStatus := 0
	rwMock := ResponseWriterMock{
		WriteHeaderMock: func(header int) {
			actualStatus = header
		},
		HeaderMock: func() http.Header {
			return http.Header{}
		},
		WriteMock: func(content []byte) (int, error) {
			json.Unmarshal(content, &actual)
			return 0, nil
		},
	}
	req, _ := http.NewRequest("GET", "/v1/docker-flow-cron/job/my-job", nil)
	expected := ResponseDetails{
		Status:     "NOK",
		Message:    message,
		Job:        cron.JobData{},
		Executions: []Execution{},
	}

	srv := Serve{Service: mock}
	srv.JobDetailsHandler(rwMock, req)

	s.Equal(expected, actual)
	s.Equal(500, actualStatus)
}

// Mock

type ResponseWriterMock struct {
	HeaderMock      func() http.Header
	WriteMock       func([]byte) (int, error)
	WriteHeaderMock func(int)
}

func (m ResponseWriterMock) Header() http.Header {
	return m.HeaderMock()
}

func (m ResponseWriterMock) Write(content []byte) (int, error) {
	return m.WriteMock(content)
}

func (m ResponseWriterMock) WriteHeader(header int) {
	m.WriteHeaderMock(header)
}

type CronerMock struct {
	AddJobMock         func(data cron.JobData) error
	StopMock           func()
	GetJobsMock        func() (map[string]cron.JobData, error)
	RemoveJobMock      func(jobName string) error
	RescheduleJobsMock func() error
}

func (m CronerMock) AddJob(data cron.JobData) error {
	return m.AddJobMock(data)
}

func (m CronerMock) Stop() {
	m.StopMock()
}

func (m CronerMock) GetJobs() (map[string]cron.JobData, error) {
	return m.GetJobsMock()
}

func (m CronerMock) RemoveJob(jobName string) error {
	return m.RemoveJobMock(jobName)
}

func (m CronerMock) RescheduleJobs() error {
	return m.RescheduleJobsMock()
}

type ServicerMock struct {
	GetServicesMock    func(jobName string) ([]swarm.Service, error)
	GetTasksMock       func(jobName string) ([]swarm.Task, error)
	RemoveServicesMock func(jobName string) error
}

func (m ServicerMock) GetServices(jobName string) ([]swarm.Service, error) {
	return m.GetServicesMock(jobName)
}

func (m ServicerMock) GetTasks(jobName string) ([]swarm.Task, error) {
	return m.GetTasksMock(jobName)
}

func (m ServicerMock) RemoveServices(jobName string) error {
	return m.RemoveServicesMock(jobName)
}
