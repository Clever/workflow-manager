package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path"

	oldmodels "github.com/Clever/old-workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/gen-go/models"
)

func convert(fullPath string) {
	//f, err := os.Open(fullPath)
	fileBytes, err := ioutil.ReadFile(fullPath)
	if err != nil {
		panic(err)
	}
	// decode into old newworkflowdefinitionrequest object
	var oldWDReq oldmodels.NewWorkflowDefinitionRequest
	if err := json.Unmarshal(fileBytes, &oldWDReq); err != nil {
		panic(err)
	}

	newWDReq := models.NewWorkflowDefinitionRequest{
		Manager: models.Manager(oldWDReq.Manager),
		Name:    oldWDReq.Name,
		StateMachine: &models.SLStateMachine{
			Comment: oldWDReq.Description,
			StartAt: oldWDReq.StartAt,
			Version: "1.0",
			States:  map[string]models.SLState{},
		},
	}
	for _, state := range oldWDReq.States {
		newWDReq.StateMachine.States[state.Name] = models.SLState{
			Comment:  state.Comment,
			End:      state.End,
			Next:     state.Next,
			Resource: state.Resource,
			Retry:    []*models.SLRetrier{}, // don't bother, none of them have it
			Type:     models.SLStateTypeTask,
		}
	}

	newFileBytes, err := json.MarshalIndent(newWDReq, "", "    ")
	if err != nil {
		panic(err)
	}

	if err := ioutil.WriteFile(fullPath+".bak", fileBytes, os.FileMode(0664)); err != nil {
		panic(err)
	}
	if err := ioutil.WriteFile(fullPath, newFileBytes, os.FileMode(0664)); err != nil {
		panic(err)
	}
}

func main() {
	parentDir := os.Getenv("GOPATH") + "/src/github.com/Clever/ark-config/workflows"
	files, err := ioutil.ReadDir(parentDir)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		fullPath := path.Join(parentDir, file.Name())
		log.Printf("converting: %s", fullPath)
		convert(fullPath)
	}

}
