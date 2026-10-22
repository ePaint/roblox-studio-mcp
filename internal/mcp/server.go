package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ePaint/roblox-studio-mcp/internal/bridge"
	"github.com/ePaint/roblox-studio-mcp/internal/host"
	"github.com/ePaint/roblox-studio-mcp/internal/types"
)

type Server struct {
	mcpServer *mcp.Server
	bridge    *bridge.Server
	host      host.Host
}

func NewServer(b *bridge.Server, h host.Host) *Server {
	impl := &mcp.Implementation{
		Name:    "roblox-studio-mcp",
		Version: "0.1.0",
	}

	opts := &mcp.ServerOptions{}
	srv := mcp.NewServer(impl, opts)

	s := &Server{
		mcpServer: srv,
		bridge:    b,
		host:      h,
	}

	s.registerTools()
	return s
}

func (s *Server) Run(ctx context.Context) error {
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}

// Tool input structs
type ExecuteLuauInput struct {
	Code     string  `json:"code" jsonschema:"Luau code string to execute in Roblox Studio"`
	Context  string  `json:"context,omitempty" jsonschema:"Execution context: edit or server"`
	TimeoutS float64 `json:"timeout_s,omitempty" jsonschema:"Execution timeout in seconds (default 30)"`
}

type DeleteInstanceInput struct {
	Target any `json:"target" jsonschema:"Instance reference (id, dotted path, or segment array)"`
}

type BuildTreeInput struct {
	Parent any `json:"parent" jsonschema:"Where the root node is parented (id, path, or segment array)"`
	Tree   any `json:"tree" jsonschema:"Declarative node tree {class, name, props, attributes, tags, children}"`
}

type TagsPayload struct {
	Add    []string `json:"add,omitempty"`
	Remove []string `json:"remove,omitempty"`
}

type SetPropertiesInput struct {
	Target     any            `json:"target" jsonschema:"Target instance reference"`
	Properties map[string]any `json:"properties,omitempty" jsonschema:"Property map in wire format"`
	Attributes map[string]any `json:"attributes,omitempty" jsonschema:"Attributes map"`
	Tags       *TagsPayload   `json:"tags,omitempty" jsonschema:"Tags to add or remove"`
}

type GetInstanceTreeInput struct {
	Target            any    `json:"target" jsonschema:"Target instance reference"`
	Depth             int    `json:"depth,omitempty" jsonschema:"Depth of descendants (default 1)"`
	IncludeProperties any    `json:"include_properties,omitempty" jsonschema:"Boolean true for all properties or array of property names"`
	MaxNodes          int    `json:"max_nodes,omitempty" jsonschema:"Maximum nodes to return (default 500)"`
	Cursor            string `json:"cursor,omitempty" jsonschema:"Pagination cursor"`
}

type FindInstancesInput struct {
	Root        any    `json:"root,omitempty" jsonschema:"Root instance to search from (default game)"`
	ClassName   string `json:"class_name,omitempty" jsonschema:"Class name filter (IsA)"`
	NamePattern string `json:"name_pattern,omitempty" jsonschema:"Lua pattern matched against Name"`
	Tag         string `json:"tag,omitempty" jsonschema:"CollectionService tag filter"`
	Limit       int    `json:"limit,omitempty" jsonschema:"Maximum matches to return (default 500)"`
}

type CallMethodInput struct {
	Target any    `json:"target" jsonschema:"Target instance reference"`
	Method string `json:"method" jsonschema:"Method name to invoke"`
	Args   []any  `json:"args,omitempty" jsonschema:"Arguments array in wire format"`
}

type CreatePhysicsJointInput struct {
	JointType  string         `json:"joint_type" jsonschema:"Joint type: Prismatic, Hinge, Weld, Spring, BallSocket, AlignPosition, AlignOrientation"`
	PartA      any            `json:"part_a" jsonschema:"First part reference"`
	PartB      any            `json:"part_b" jsonschema:"Second part reference"`
	Properties map[string]any `json:"properties,omitempty" jsonschema:"Joint properties"`
}

type ManageScriptInput struct {
	Action     string `json:"action" jsonschema:"Script action: read, write, or delete"`
	Path       any    `json:"path" jsonschema:"Script path or reference"`
	Parent     any    `json:"parent,omitempty" jsonschema:"Parent instance when creating a new script"`
	Content    string `json:"content,omitempty" jsonschema:"Source code for write"`
	ScriptType string `json:"script_type,omitempty" jsonschema:"Script class type: Script, LocalScript, or ModuleScript"`
	RunContext string `json:"run_context,omitempty" jsonschema:"RunContext for Script: Server, Client, or Plugin"`
}

type InsertAssetInput struct {
	AssetID int64 `json:"asset_id" jsonschema:"Creator Store model asset ID"`
	Parent  any   `json:"parent,omitempty" jsonschema:"Parent instance reference"`
}

type GetConsoleLogsInput struct {
	SinceSeq int64  `json:"since_seq,omitempty" jsonschema:"Return logs after this sequence number"`
	Level    string `json:"level,omitempty" jsonschema:"Filter by log level: output, info, warning, error"`
	Source   string `json:"source,omitempty" jsonschema:"Filter by log source: edit, server, client"`
	Pattern  string `json:"pattern,omitempty" jsonschema:"Lua pattern or text substring filter"`
	WaitMS   int    `json:"wait_ms,omitempty" jsonschema:"Milliseconds to wait for incoming logs before returning"`
}

type CaptureViewportInput struct {
	MaxWidth int `json:"max_width,omitempty" jsonschema:"Maximum width in pixels for the captured image (default 1568)"`
}

type StudioInput struct {
	Action string `json:"action" jsonschema:"Studio action to perform: status, selection, save"`
}

func (s *Server) registerTools() {
	dispatch := func(ctx context.Context, tool string, in any, timeout time.Duration) (*mcp.CallToolResult, any, error) {
		res, err := s.bridge.DispatchJob(ctx, tool, in, timeout)
		if err != nil {
			return nil, nil, err
		}
		if !res.OK && res.Error != nil {
			return &mcp.CallToolResult{IsError: true}, res.Error, nil
		}
		var val any
		if len(res.Value) > 0 {
			json.Unmarshal(res.Value, &val)
		}
		return nil, val, nil
	}

	// 1. execute_luau
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "execute_luau",
		Description: "Executes arbitrary Luau code in Roblox Studio and returns the results.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in ExecuteLuauInput) (*mcp.CallToolResult, any, error) {
		timeout := 30 * time.Second
		if in.TimeoutS > 0 {
			timeout = time.Duration(in.TimeoutS * float64(time.Second))
		}
		return dispatch(ctx, "execute_luau", in, timeout)
	})

	// 2. build_tree
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "build_tree",
		Description: "Builds a complete instance hierarchy or responsive UI tree in a single call.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in BuildTreeInput) (*mcp.CallToolResult, any, error) {
		return dispatch(ctx, "build_tree", in, 30*time.Second)
	})

	// 3. set_properties
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "set_properties",
		Description: "Sets properties, attributes, and CollectionService tags on any instance.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in SetPropertiesInput) (*mcp.CallToolResult, any, error) {
		return dispatch(ctx, "set_properties", in, 20*time.Second)
	})

	// 4. get_instance_tree
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_instance_tree",
		Description: "Traverses the DataModel scene graph starting from the target instance.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in GetInstanceTreeInput) (*mcp.CallToolResult, any, error) {
		return dispatch(ctx, "get_instance_tree", in, 20*time.Second)
	})

	// 5. find_instances
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "find_instances",
		Description: "Searches descendants by class name, name pattern, tag, or attribute.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in FindInstancesInput) (*mcp.CallToolResult, any, error) {
		return dispatch(ctx, "find_instances", in, 20*time.Second)
	})

	// 6. delete_instance
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "delete_instance",
		Description: "Removes an instance cleanly from the Roblox DataModel.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in DeleteInstanceInput) (*mcp.CallToolResult, any, error) {
		return dispatch(ctx, "delete_instance", in, 15*time.Second)
	})

	// 7. call_method
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "call_method",
		Description: "Directly invokes an engine method on an instance (PivotTo, ApplyImpulse, Clone, etc.).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in CallMethodInput) (*mcp.CallToolResult, any, error) {
		return dispatch(ctx, "call_method", in, 20*time.Second)
	})

	// 8. create_physics_joint
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "create_physics_joint",
		Description: "Universal physics and mechanical constraint builder (Prismatic, Hinge, Weld, Spring, AlignPosition).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in CreatePhysicsJointInput) (*mcp.CallToolResult, any, error) {
		return dispatch(ctx, "create_physics_joint", in, 20*time.Second)
	})

	// 9. manage_script
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "manage_script",
		Description: "Reads, writes, or deletes Luau scripts using ScriptEditorService to prevent open-tab desync.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in ManageScriptInput) (*mcp.CallToolResult, any, error) {
		return dispatch(ctx, "manage_script", in, 20*time.Second)
	})

	// 10. insert_asset
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "insert_asset",
		Description: "Loads a 3D model from the Creator Store via InsertService.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in InsertAssetInput) (*mcp.CallToolResult, any, error) {
		return dispatch(ctx, "insert_asset", in, 30*time.Second)
	})

	// 11. get_console_logs
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_console_logs",
		Description: "Retrieves captured console output, warnings, and error logs across Edit and Play sessions.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in GetConsoleLogsInput) (*mcp.CallToolResult, any, error) {
		timeout := time.Duration(in.WaitMS) * time.Millisecond
		entries, nextSeq := s.bridge.Logs().Query(in.SinceSeq, in.Level, in.Source, in.Pattern, timeout)
		return nil, map[string]any{
			"entries":  entries,
			"next_seq": nextSeq,
		}, nil
	})

	// 12. capture_viewport
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "capture_viewport",
		Description: "Takes an OS-level window screenshot of the active Roblox Studio viewport for visual inspection.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in CaptureViewportInput) (*mcp.CallToolResult, any, error) {
		if !s.host.IsSupported() {
			return &mcp.CallToolResult{IsError: true}, &types.ErrorInfo{
				Code:    types.ErrUnsupportedHost,
				Message: "viewport capture requires a Windows host",
			}, nil
		}

		maxWidth := in.MaxWidth
		if maxWidth <= 0 {
			maxWidth = 1568
		}

		pngBytes, err := s.host.CaptureViewport(maxWidth)
		if err != nil {
			return &mcp.CallToolResult{IsError: true}, &types.ErrorInfo{
				Code:    types.ErrNotConnected,
				Message: fmt.Sprintf("failed to capture viewport: %v", err),
			}, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.ImageContent{
					Data:     pngBytes,
					MIMEType: "image/png",
				},
			},
		}, nil, nil
	})

	// 13. studio
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "studio",
		Description: "Checks Studio connection status, reads active selection, or triggers a place save.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in StudioInput) (*mcp.CallToolResult, any, error) {
		switch in.Action {
		case "status":
			status := s.bridge.GetStatus()
			if s.host.IsSupported() {
				if _, err := s.host.FindStudioWindow(); err == nil {
					status.StudioRunning = true
				}
			}
			return nil, status, nil

		case "save":
			if !s.host.IsSupported() {
				return &mcp.CallToolResult{IsError: true}, &types.ErrorInfo{
					Code:    types.ErrUnsupportedHost,
					Message: "save place requires a Windows host",
				}, nil
			}
			if err := s.host.SavePlace(); err != nil {
				return &mcp.CallToolResult{IsError: true}, &types.ErrorInfo{
					Code:    types.ErrNeedsManualSave,
					Message: fmt.Sprintf("save failed: %v", err),
				}, nil
			}
			return nil, map[string]any{"ok": true}, nil

		default:
			return &mcp.CallToolResult{IsError: true}, &types.ErrorInfo{
				Code:    types.ErrCoercion,
				Message: fmt.Sprintf("unsupported studio action: %s", in.Action),
			}, nil
		}
	})
}
