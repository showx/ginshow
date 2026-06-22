package ginshow

import (
	"bytes"
	"fmt"
	"net/http"
	"runtime/pprof"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/pprof/profile"
)

// FlameNode is a node in the flame graph tree returned by the flame API.
type FlameNode struct {
	Name     string      `json:"name"`
	Value    int64       `json:"value"`
	Children []FlameNode `json:"children,omitempty"`
}

type flameResponse struct {
	Type  string    `json:"type"`
	Unit  string    `json:"unit"`
	Total int64     `json:"total"`
	Root  FlameNode `json:"root"`
}

type flameSpec struct {
	lookup     string
	samplePref []string
	unit       string
}

var flameSpecs = map[string]flameSpec{
	"cpu":       {lookup: "cpu", samplePref: []string{"samples", "cpu"}, unit: "samples"},
	"heap":      {lookup: "heap", samplePref: []string{"inuse_space", "alloc_space"}, unit: "bytes"},
	"goroutine": {lookup: "goroutine", samplePref: []string{"goroutine"}, unit: "goroutines"},
	"allocs":    {lookup: "allocs", samplePref: []string{"alloc_space", "alloc_objects"}, unit: "bytes"},
	"block":     {lookup: "block", samplePref: []string{"contentions", "delay"}, unit: "contentions"},
	"mutex":     {lookup: "mutex", samplePref: []string{"contentions", "delay"}, unit: "contentions"},
}

func flameHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		profileType := c.DefaultQuery("type", "heap")
		spec, ok := flameSpecs[profileType]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("unsupported profile type %q", profileType),
			})
			return
		}

		seconds := clampFlameSeconds(c.Query("seconds"))

		raw, err := collectProfileBytes(spec.lookup, seconds)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		p, err := profile.Parse(bytes.NewReader(raw))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "parse profile: " + err.Error()})
			return
		}

		if err := p.CheckValid(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid profile: " + err.Error()})
			return
		}

		p.RemoveUninteresting()
		root := buildFlameTree(p, sampleTypeIndex(p, spec.samplePref...))
		if root.Value == 0 {
			c.JSON(http.StatusOK, flameResponse{
				Type:  profileType,
				Unit:  spec.unit,
				Total: 0,
				Root:  FlameNode{Name: "root", Value: 0},
			})
			return
		}

		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, flameResponse{
			Type:  profileType,
			Unit:  spec.unit,
			Total: root.Value,
			Root:  root,
		})
	}
}

func clampFlameSeconds(raw string) int {
	seconds := 10
	if raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			seconds = v
		}
	}
	if seconds < 1 {
		return 1
	}
	if seconds > 120 {
		return 120
	}
	return seconds
}

func collectProfileBytes(profileType string, seconds int) ([]byte, error) {
	if profileType == "cpu" {
		var buf bytes.Buffer
		if err := pprof.StartCPUProfile(&buf); err != nil {
			return nil, fmt.Errorf("start cpu profile: %w", err)
		}
		time.Sleep(time.Duration(seconds) * time.Second)
		pprof.StopCPUProfile()
		return buf.Bytes(), nil
	}

	prof := pprof.Lookup(profileType)
	if prof == nil {
		return nil, fmt.Errorf("profile %q not found", profileType)
	}

	var buf bytes.Buffer
	if err := prof.WriteTo(&buf, 0); err != nil {
		return nil, fmt.Errorf("write profile: %w", err)
	}
	return buf.Bytes(), nil
}

func sampleTypeIndex(p *profile.Profile, prefer ...string) int {
	for _, name := range prefer {
		for i, st := range p.SampleType {
			if st.Type == name {
				return i
			}
		}
	}
	return 0
}

type flameBuilderNode struct {
	name     string
	value    int64
	children map[string]*flameBuilderNode
}

func buildFlameTree(p *profile.Profile, sampleIndex int) FlameNode {
	root := &flameBuilderNode{name: "root", children: make(map[string]*flameBuilderNode)}

	for _, sample := range p.Sample {
		if sampleIndex >= len(sample.Value) {
			continue
		}
		value := sample.Value[sampleIndex]
		if value <= 0 {
			continue
		}

		node := root
		node.value += value

		for i := len(sample.Location) - 1; i >= 0; i-- {
			name := frameName(sample.Location[i])
			node = node.child(name)
			node.value += value
		}
	}

	return root.toFlameNode()
}

func (n *flameBuilderNode) child(name string) *flameBuilderNode {
	if name == "" {
		name = "unknown"
	}
	if n.children == nil {
		n.children = make(map[string]*flameBuilderNode)
	}
	child, ok := n.children[name]
	if !ok {
		child = &flameBuilderNode{name: name, children: make(map[string]*flameBuilderNode)}
		n.children[name] = child
	}
	return child
}

func (n *flameBuilderNode) toFlameNode() FlameNode {
	children := make([]FlameNode, 0, len(n.children))
	for _, child := range n.children {
		children = append(children, child.toFlameNode())
	}
	sort.Slice(children, func(i, j int) bool {
		if children[i].Value == children[j].Value {
			return children[i].Name < children[j].Name
		}
		return children[i].Value > children[j].Value
	})
	return FlameNode{
		Name:     n.name,
		Value:    n.value,
		Children: children,
	}
}

func frameName(loc *profile.Location) string {
	if loc == nil {
		return "unknown"
	}
	for i := len(loc.Line) - 1; i >= 0; i-- {
		if fn := loc.Line[i].Function; fn != nil {
			if fn.Name != "" {
				return fn.Name
			}
			if fn.Filename != "" {
				return fn.Filename
			}
		}
	}
	if loc.Mapping != nil && loc.Mapping.File != "" {
		return loc.Mapping.File
	}
	return fmt.Sprintf("L%d", loc.ID)
}
