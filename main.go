package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"text/template"

	"golang.org/x/exp/maps"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type Event struct {
	Name string
	From []string
	To   string
}

func (e Event) NameGo() string {
	return eventName(e.Name)
}

func (e Event) ToGo() string {
	return stateName(e.To)
}

type State struct {
	Name string
}

func (s State) NameGo() string {
	return stateName(s.Name)
}

type Input struct {
	Events       []Event
	States       []State
	InitialState State
	Package      string
	TypeName     string
}

func (i Input) StateName(s string) string {
	return stateName(s)
}

type srcState struct {
	On   map[string]string `json:"on,omitempty"`
	Type string            `json:"type,omitempty"`
}

type FSMData struct {
	Id          string              `json:"id,omitempty"`
	Initial     string              `json:"initial,omitempty"`
	Description string              `json:"descriptions,omitempty"`
	States      map[string]srcState `json:"states,omitempty"`
}

func main() {
	fmt.Printf("Running %s go on %s\n", os.Args[0], os.Getenv("GOFILE"))

	content, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal("Error when opening file: ", err)
	}

	fsm := FSMData{}

	err = json.Unmarshal([]byte(content), &fsm)
	if err != nil {
		log.Fatal("Error when parsing json: ", err)
	}

	fmt.Printf("creating state machine with id %s, initial state %s, and description: %s", fsm.Id, fsm.Initial, fsm.Description)

	states := map[string]bool{}
	events := map[string]Event{}

	for k, v := range fsm.States {
		states[k] = true
		for ek, ev := range v.On {
			e, ok := events[ek]
			if !ok {
				e = Event{
					Name: ek,
					From: []string{k},
					To:   ev,
				}
			} else {
				// current implementation limitation, one event could be sourced from different states, but lead to only one
				if ev != e.To {
					log.Fatalf("Event %s has two destinations, %s and %s which is not supported 😢.", k, ev, e.To)
				}
				e.From = append(e.From, k)
			}
			events[ek] = e
		}
	}

	g := Generator{}

	tt, err := template.New("gen").Parse(tmpl)
	if err != nil {
		panic(err)
	}

	statesList := []State{}
	for s := range states {
		if s != fsm.Initial {
			statesList = append(statesList, State{Name: s})
		}
	}

	pkg := os.Getenv("GOPACKAGE")
	if len(pkg) == 0 {
		pkg = "main"
	}

	data := Input{
		Events:       maps.Values(events),
		States:       statesList,
		Package:      pkg,
		TypeName:     os.Args[2],
		InitialState: State{Name: fsm.Initial},
	}
	err = tt.Execute(&g.buf, data)
	if err != nil {
		log.Fatalf("templating error: %s", err)
	}
	// g.p(template)

	destinationFile := strings.TrimRight(os.Args[1], ".json")
	destinationFile = destinationFile + ".go"

	src := g.format()
	err = os.WriteFile(destinationFile, src, 0o644)
	if err != nil {
		log.Fatalf("writing output: %s", err)
	}
}

func stateName(s string) string {
	return fmt.Sprintf("%sState", gName(s))
}

func eventName(s string) string {
	return fmt.Sprintf("%sEvent", gName(s))
}

func gName(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ToLower(s)
	s = cases.Title(language.Und).String(s)
	s = strings.ReplaceAll(s, " ", "")
	return s
}

var tmpl = `
// Code generated by "fsmfy.go"; DO NOT EDIT.
package {{ .Package }}

import (
	"context"

	"github.com/looplab/fsm"
)

type (
	// {{ .TypeName }}State represents all states for this FSM.
	{{ .TypeName }}State string
	//{{ .TypeName }}Event  represents all events for this FSM.
	{{ .TypeName }}Event string
)

const (
	// States.
	// {{ .InitialState.NameGo }} is initial state.
	{{ .InitialState.NameGo }} {{ .TypeName }}State = "{{ .InitialState.Name }}"
	{{ range .States }} 
	// {{ .NameGo }} state.
	{{ .NameGo }} {{ $.TypeName }}State = "{{ .Name }}" {{ end }}

	//Events.
	{{ range .Events }}
	// {{ .NameGo }} state.
	{{ .NameGo }} {{ $.TypeName }}Event = "{{ .Name }}"{{ end }}
)

//{{ .TypeName }}  implements FSM.
type {{ .TypeName }} struct {
	fsm *fsm.FSM
}

//New{{ .TypeName }} creates new FSM with callbacks provided.
func New{{ .TypeName }}(callbacks fsm.Callbacks) *{{ .TypeName }} {
	fsm := fsm.NewFSM(
		{{ .InitialState.NameGo }}.String(),
		fsm.Events{
			{{ range .Events }}
				{ Name: {{ .NameGo }}.String(), Src:[]string{ {{ range $i, $e := .From }}{{ if $i }}, {{ end }}{{ $.StateName $e }}.String(){{ end }} }, Dst: {{ .ToGo }}.String() },  {{ end }}			
		},
		callbacks,
	)
	return &{{ .TypeName }}{fsm: fsm}
}

// String returns string representation of the state.
func (s {{ .TypeName }}State) String() string {
	return string(s)
}

// String returns string representation of the event.
func (s {{ .TypeName }}Event) String() string {
	return string(s)
}

// Current returns the current state of the {{ .TypeName }}.
func (f *{{ .TypeName }}) Current() {{ .TypeName }}State {
	return {{ .TypeName }}State(f.fsm.Current())
}

// Is returns true if state is the current state.
func (f *{{ .TypeName }}) Is(state {{ .TypeName }}State) bool {
	return f.fsm.Is(state.String())
}

// SetState allows the user to move to the given state from current state.
// The call does not trigger any callbacks, if defined.
func (f *{{ .TypeName }}) SetState(state {{ .TypeName }}State) {
	f.fsm.SetState(state.String())
}

// Can returns true if event can occur in the current state.
func (f *{{ .TypeName }}) Can(event {{ .TypeName }}Event) bool {
	return f.fsm.Can(event.String())
}

// AvailableTransitions returns a list of transitions available in the
// current state.
func (f *{{ .TypeName }}) AvailableTransitions() []string {
	return f.fsm.AvailableTransitions()
}

// Cannot returns true if event can not occur in the current state.
// It is a convenience method to help code read nicely.
func (f *{{ .TypeName }}) Cannot(event {{ .TypeName }}Event) bool {
	return f.fsm.Cannot(event.String())
}

// Metadata returns the value stored in metadata
func (f *{{ .TypeName }}) Metadata(key string) (interface{}, bool) {
	return f.fsm.Metadata(key)
}

// SetMetadata stores the dataValue in metadata indexing it with key
func (f *{{ .TypeName }}) SetMetadata(key string, dataValue interface{}) {
	f.fsm.SetMetadata(key, dataValue)
}

// DeleteMetadata deletes the dataValue in metadata by key
func (f *{{ .TypeName }}) DeleteMetadata(key string) {
	f.fsm.DeleteMetadata(key)
}

// Event initiates a state transition with the named event.
//
// The call takes a variable number of arguments that will be passed to the
// callback, if defined.
func (f *{{ .TypeName }}) Event(ctx context.Context, event {{ .TypeName }}Event, args ...interface{}) error {
	return f.fsm.Event(ctx, event.String(), args...)
}
`
