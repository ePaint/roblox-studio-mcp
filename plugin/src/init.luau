--!strict
-- Roblox Studio MCP Companion Plugin
-- Complete two-layer tool executor and HTTP bridge connector

local RunService = game:GetService("RunService")
local HttpService = game:GetService("HttpService")
local ChangeHistoryService = game:GetService("ChangeHistoryService")
local ScriptEditorService = game:GetService("ScriptEditorService")
local LogService = game:GetService("LogService")
local CollectionService = game:GetService("CollectionService")
local InsertService = game:GetService("InsertService")

-- Only run the polling bridge in Edit mode
if not RunService:IsEdit() then
	return
end

local DEFAULT_BRIDGE_URL = "http://127.0.0.1:58741"
local SETTING_TOKEN_KEY = "ROBLOX_STUDIO_MCP_TOKEN"
local SETTING_URL_KEY = "ROBLOX_STUDIO_MCP_URL"
local SESSION_ID = HttpService:GenerateGUID(false):sub(1, 8)

local token: string? = plugin:GetSetting(SETTING_TOKEN_KEY)
local savedUrl: string? = plugin:GetSetting(SETTING_URL_KEY)
local bridgeUrl = if savedUrl and #savedUrl > 0 then savedUrl else DEFAULT_BRIDGE_URL
local isPolling = false
local pendingLogs = {}

local function normalizeUrl(raw: string): string
	local trimmed = raw:gsub("^%s+", ""):gsub("%s+$", "")
	if trimmed == "" then
		return DEFAULT_BRIDGE_URL
	end
	if not (trimmed:match("^http://") or trimmed:match("^https://")) then
		trimmed = "http://" .. trimmed
	end
	if trimmed:sub(-1) == "/" then
		trimmed = trimmed:sub(1, -2)
	end
	return trimmed
end

--------------------------------------------------------------------------------
-- Type Deserializer & Serializer
--------------------------------------------------------------------------------
local function resolveInstance(ref: any): Instance?
	if typeof(ref) == "string" then
		if ref:sub(1, 5) == "game." then
			local segments = ref:split(".")
			local current: any = game
			for i = 2, #segments do
				current = current:FindFirstChild(segments[i])
				if not current then return nil end
			end
			return current
		end

		for _, desc in ipairs(game:GetDescendants()) do
			if desc.Name == ref or desc:GetDebugId() == ref then
				return desc
			end
		end
	elseif typeof(ref) == "table" and #ref > 0 then
		local current: any = game
		for _, seg in ipairs(ref) do
			local name, indexStr = string.match(seg, "^([^#]+)#(%d+)$")
			if name and indexStr then
				local targetIdx = tonumber(indexStr) or 1
				local count = 0
				local matched: Instance? = nil
				for _, child in ipairs(current:GetChildren()) do
					if child.Name == name then
						count += 1
						if count == targetIdx then
							matched = child
							break
						end
					end
				end
				current = matched
			else
				current = current:FindFirstChild(seg)
			end
			if not current then return nil end
		end
		return current
	end
	return nil
end

local function deserialize(val: any, targetPropType: string?): (boolean, any, string?)
	if val == nil then return true, nil, nil end
	local valType = typeof(val)
	if valType == "number" or valType == "string" or valType == "boolean" then
		return true, val, nil
	end

	if valType == "table" then
		local t = val["$t"]
		if t then
			if t == "Vector3" and val.v then
				return true, Vector3.new(val.v[1] or 0, val.v[2] or 0, val.v[3] or 0), nil
			elseif t == "Vector2" and val.v then
				return true, Vector2.new(val.v[1] or 0, val.v[2] or 0), nil
			elseif t == "UDim" and val.v then
				return true, UDim.new(val.v[1] or 0, val.v[2] or 0), nil
			elseif t == "UDim2" and val.v then
				return true, UDim2.new(val.v[1] or 0, val.v[2] or 0, val.v[3] or 0, val.v[4] or 0), nil
			elseif t == "Color3" and val.rgb then
				return true, Color3.fromRGB(val.rgb[1] or 0, val.rgb[2] or 0, val.rgb[3] or 0), nil
			elseif t == "CFrame" then
				if val.v and #val.v == 12 then
					return true, CFrame.new(table.unpack(val.v)), nil
				elseif val.pos then
					local cf = CFrame.new(val.pos[1] or 0, val.pos[2] or 0, val.pos[3] or 0)
					if val.rot_deg then
						local rx = math.rad(val.rot_deg[1] or 0)
						local ry = math.rad(val.rot_deg[2] or 0)
						local rz = math.rad(val.rot_deg[3] or 0)
						cf = cf * CFrame.Angles(rx, ry, rz)
					end
					return true, cf, nil
				end
			elseif t == "Enum" and val.v then
				local parts = string.split(val.v, ".")
				if #parts == 2 then
					local enumGroup = (Enum :: any)[parts[1]]
					if enumGroup and enumGroup[parts[2]] then
						return true, enumGroup[parts[2]], nil
					end
				end
				return false, nil, "unknown enum: " .. tostring(val.v)
			elseif t == "Instance" and val.ref then
				local inst = resolveInstance(val.ref)
				if inst then return true, inst, nil end
				return false, nil, "instance not found: " .. tostring(val.ref)
			end
		end

		if targetPropType == "Vector3" and #val == 3 then
			return true, Vector3.new(val[1], val[2], val[3]), nil
		elseif targetPropType == "UDim2" and #val == 4 then
			return true, UDim2.new(val[1], val[2], val[3], val[4]), nil
		elseif targetPropType == "Color3" and #val == 3 then
			return true, Color3.fromRGB(val[1], val[2], val[3]), nil
		end
	end

	return true, val, nil
end

local function serialize(val: any): any
	if val == nil then return nil end
	local t = typeof(val)
	if t == "number" or t == "string" or t == "boolean" then
		return val
	elseif t == "Vector3" then
		return { ["$t"] = "Vector3", v = { val.X, val.Y, val.Z } }
	elseif t == "Vector2" then
		return { ["$t"] = "Vector2", v = { val.X, val.Y } }
	elseif t == "UDim" then
		return { ["$t"] = "UDim", v = { val.Scale, val.Offset } }
	elseif t == "UDim2" then
		return { ["$t"] = "UDim2", v = { val.X.Scale, val.X.Offset, val.Y.Scale, val.Y.Offset } }
	elseif t == "Color3" then
		return { ["$t"] = "Color3", rgb = { math.round(val.R * 255), math.round(val.G * 255), math.round(val.B * 255) } }
	elseif t == "CFrame" then
		return { ["$t"] = "CFrame", v = { val:GetComponents() } }
	elseif t == "EnumItem" then
		return { ["$t"] = "Enum", v = tostring(val.EnumType) .. "." .. val.Name }
	elseif t == "Instance" then
		return { ["$t"] = "Instance", ref = val:GetDebugId(), path = val:GetFullName() }
	end
	return tostring(val)
end

--------------------------------------------------------------------------------
-- Recursive Tree Builder (build_tree)
--------------------------------------------------------------------------------
local function buildNode(parent: Instance, node: any, createdMap: { [string]: any }): Instance
	local className = node.class or "Part"
	local inst = Instance.new(className)
	if node.name then
		inst.Name = node.name
	end

	-- Apply properties
	if node.props then
		for prop, rawVal in pairs(node.props) do
			local ok, val = deserialize(rawVal, nil)
			if ok then
				pcall(function()
					(inst :: any)[prop] = val
				end)
			end
		end
	end

	-- Apply attributes
	if node.attributes then
		for k, v in pairs(node.attributes) do
			inst:SetAttribute(k, v)
		end
	end

	-- Apply tags
	if node.tags then
		for _, tag in ipairs(node.tags) do
			CollectionService:AddTag(inst, tag)
		end
	end

	-- UI Sugar expansions
	if node.corner and typeof(node.corner) == "number" then
		local c = Instance.new("UICorner")
		c.CornerRadius = UDim.new(0, node.corner)
		c.Parent = inst
	end
	if node.stroke and typeof(node.stroke) == "number" then
		local s = Instance.new("UIStroke")
		s.Thickness = node.stroke
		s.Parent = inst
	end
	if node.aspect_ratio and typeof(node.aspect_ratio) == "number" then
		local ar = Instance.new("UIAspectRatioConstraint")
		ar.AspectRatio = node.aspect_ratio
		ar.Parent = inst
	end
	if node.auto_canvas then
		pcall(function()
			(inst :: any).AutomaticCanvasSize = Enum.AutomaticSize.Y
		end)
	end

	inst.Parent = parent

	if node.name then
		createdMap[node.name] = { id = inst:GetDebugId(), path = inst:GetFullName() }
	end

	-- Build children
	if node.children then
		for _, childNode in ipairs(node.children) do
			buildNode(inst, childNode, createdMap)
		end
	end

	return inst
end

--------------------------------------------------------------------------------
-- Job Router Implementation
--------------------------------------------------------------------------------
local function executeJob(job: any): (boolean, any, any)
	local tool = job.tool
	local args = job.args or {}

	if tool == "execute_luau" then
		local code = args.code
		if not code then
			return false, nil, { code = "COERCION", message = "missing 'code' argument" }
		end
		local fn, compileErr = loadstring(code)
		if not fn then
			return false, nil, { code = "LUAU_ERROR", message = "compile error: " .. tostring(compileErr) }
		end
		local ok, result = pcall(fn)
		if not ok then
			return false, nil, { code = "LUAU_ERROR", message = tostring(result) }
		end
		return true, serialize(result), nil

	elseif tool == "build_tree" then
		local parentInst = resolveInstance(args.parent)
		if not parentInst then
			return false, nil, { code = "STALE_REF", message = "parent instance not found" }
		end
		local createdNodes = {}
		local ok, err = pcall(buildNode, parentInst, args.tree, createdNodes)
		if not ok then
			return false, nil, { code = "LUAU_ERROR", message = tostring(err) }
		end
		return true, { nodes = createdNodes }, nil

	elseif tool == "set_properties" then
		local inst = resolveInstance(args.target)
		if not inst then
			return false, nil, { code = "STALE_REF", message = "target instance not found" }
		end

		local applied = {}
		local failed = {}

		if args.properties then
			for prop, rawVal in pairs(args.properties) do
				local okDeser, val, deserErr = deserialize(rawVal, nil)
				if not okDeser then
					failed[prop] = deserErr or "deserialization failed"
				else
					local okSet, setErr = pcall(function()
						(inst :: any)[prop] = val
					end)
					if okSet then
						table.insert(applied, prop)
					else
						failed[prop] = tostring(setErr)
					end
				end
			end
		end

		if args.attributes then
			for k, v in pairs(args.attributes) do
				inst:SetAttribute(k, v)
			end
		end

		if args.tags then
			if args.tags.add then
				for _, t in ipairs(args.tags.add) do CollectionService:AddTag(inst, t) end
			end
			if args.tags.remove then
				for _, t in ipairs(args.tags.remove) do CollectionService:RemoveTag(inst, t) end
			end
		end

		return true, { applied = applied, failed = failed }, nil

	elseif tool == "get_instance_tree" then
		local inst = resolveInstance(args.target)
		if not inst then
			return false, nil, { code = "STALE_REF", message = "target instance not found" }
		end

		local depth = args.depth or 1
		local maxNodes = args.max_nodes or 500
		local count = 0

		local function readTree(target: Instance, currentDepth: number): any
			count += 1
			if count > maxNodes then return nil end

			local nodeData: any = {
				id = target:GetDebugId(),
				name = target.Name,
				class = target.ClassName,
				path = target:GetFullName(),
			}

			if currentDepth < depth then
				local childList = {}
				for _, child in ipairs(target:GetChildren()) do
					local c = readTree(child, currentDepth + 1)
					if c then table.insert(childList, c) end
				end
				nodeData.children = childList
			end
			return nodeData
		end

		local tree = readTree(inst, 0)
		return true, { nodes = tree }, nil

	elseif tool == "find_instances" then
		local root = if args.root then resolveInstance(args.root) else game
		if not root then return false, nil, { code = "STALE_REF", message = "root instance not found" } end

		local limit = args.limit or 100
		local matches = {}

		for _, desc in ipairs(root:GetDescendants()) do
			if #matches >= limit then break end
			local isMatch = true

			if args.class_name and not desc:IsA(args.class_name) then isMatch = false end
			if isMatch and args.tag and not CollectionService:HasTag(desc, args.tag) then isMatch = false end
			if isMatch and args.name_pattern and not string.match(desc.Name, args.name_pattern) then isMatch = false end

			if isMatch then
				table.insert(matches, { id = desc:GetDebugId(), path = desc:GetFullName(), class = desc.ClassName })
			end
		end

		return true, matches, nil

	elseif tool == "delete_instance" then
		local inst = resolveInstance(args.target)
		if not inst then
			return false, nil, { code = "STALE_REF", message = "target instance not found" }
		end
		local path = inst:GetFullName()
		local id = inst:GetDebugId()
		inst:Destroy()
		return true, { deleted = { id = id, path = path } }, nil

	elseif tool == "call_method" then
		local inst = resolveInstance(args.target)
		if not inst then
			return false, nil, { code = "STALE_REF", message = "target instance not found" }
		end
		local method = (inst :: any)[args.method]
		if typeof(method) ~= "function" then
			return false, nil, { code = "COERCION", message = "method not found: " .. tostring(args.method) }
		end

		local deserializedArgs = {}
		if args.args then
			for _, a in ipairs(args.args) do
				local _, argVal = deserialize(a, nil)
				table.insert(deserializedArgs, argVal)
			end
		end

		local ok, result = pcall(method, inst, table.unpack(deserializedArgs))
		if not ok then
			return false, nil, { code = "LUAU_ERROR", message = tostring(result) }
		end
		return true, { value = serialize(result) }, nil

	elseif tool == "create_physics_joint" then
		local partA = resolveInstance(args.part_a)
		local partB = resolveInstance(args.part_b)
		if not (partA and partA:IsA("BasePart") and partB and partB:IsA("BasePart")) then
			return false, nil, { code = "STALE_REF", message = "part_a or part_b is invalid BasePart" }
		end

		local jointType = args.joint_type or "Weld"
		local props = args.properties or {}

		local attA = Instance.new("Attachment")
		attA.Name = "Attachment_" .. jointType .. "_A"
		attA.Parent = partA

		local attB = Instance.new("Attachment")
		attB.Name = "Attachment_" .. jointType .. "_B"
		attB.Parent = partB

		local joint: Instance
		if jointType == "Weld" then
			local w = Instance.new("WeldConstraint")
			w.Part0 = partA
			w.Part1 = partB
			joint = w
		else
			local c = Instance.new(jointType .. "Constraint") :: any
			c.Attachment0 = attA
			c.Attachment1 = attB
			if props.actuator and c:IsA("PrismaticConstraint") then
				c.ActuatorType = Enum.ActuatorType[props.actuator] or Enum.ActuatorType.None
			end
			if props.speed then c.Speed = props.speed end
			if props.lower and props.upper then
				c.LimitsEnabled = true
				c.LowerLimit = props.lower
				c.UpperLimit = props.upper
			end
			joint = c
		end

		joint.Parent = partA

		if props.anchored_side == "a" then
			partA.Anchored = true
			partB.Anchored = false
		elseif props.anchored_side == "b" then
			partB.Anchored = true
			partA.Anchored = false
		end

		return true, {
			joint = { id = joint:GetDebugId(), path = joint:GetFullName() },
			attachments = { { id = attA:GetDebugId(), path = attA:GetFullName() }, { id = attB:GetDebugId(), path = attB:GetFullName() } }
		}, nil

	elseif tool == "manage_script" then
		local action = args.action or "read"
		local scriptPath = args.path

		if action == "read" then
			local inst = resolveInstance(scriptPath)
			if not (inst and inst:IsA("LuaSourceContainer")) then
				return false, nil, { code = "STALE_REF", message = "script not found" }
			end
			return true, {
				id = inst:GetDebugId(),
				path = inst:GetFullName(),
				source = (inst :: any).Source,
			}, nil

		elseif action == "write" then
			local inst = resolveInstance(scriptPath)
			local source = args.content or ""
			if not inst then
				local scriptType = args.script_type or "Script"
				local created = Instance.new(scriptType)
				created.Name = typeof(scriptPath) == "string" and scriptPath:match("([^.]+)$") or "NewScript"
				local parent = workspace
				if args.parent then
					parent = resolveInstance(args.parent) or workspace
				end
				created.Parent = parent
				inst = created
			end

			if inst and inst:IsA("LuaSourceContainer") then
				pcall(function()
					ScriptEditorService:UpdateSourceAsync(inst :: any, function() return source end)
				end)
				return true, { id = inst:GetDebugId(), path = inst:GetFullName() }, nil
			end
			return false, nil, { code = "LUAU_ERROR", message = "failed to write script" }
		end

	elseif tool == "insert_asset" then
		local assetId = tonumber(args.asset_id)
		local parent = if args.parent then resolveInstance(args.parent) else workspace
		if not assetId or not parent then
			return false, nil, { code = "COERCION", message = "invalid asset_id or parent" }
		end

		local ok, model = pcall(function()
			return InsertService:LoadAsset(assetId)
		end)
		if not ok or not model then
			return false, nil, { code = "LUAU_ERROR", message = "failed to load asset: " .. tostring(model) }
		end
		model.Parent = parent
		return true, { id = model:GetDebugId(), path = model:GetFullName() }, nil

	else
		return false, nil, { code = "UNKNOWN", message = "unimplemented tool: " .. tostring(tool) }
	end

	return false, nil, { code = "UNKNOWN", message = "unknown execution path" }
end

--------------------------------------------------------------------------------
-- UI Setup & Polling Loop
--------------------------------------------------------------------------------
local toolbar = plugin:CreateToolbar("Roblox MCP")
local toggleBtn = toolbar:CreateButton("MCP Status", "Configure MCP connection", "rbxassetid://15286419799")
local widgetInfo = DockWidgetPluginGuiInfo.new(Enum.InitialDockState.Float, false, false, 320, 220, 260, 180)
local widget = plugin:CreateDockWidgetPluginGui("RobloxStudioMCP_Config", widgetInfo)
widget.Title = "Roblox Studio MCP Bridge"

local mainFrame = Instance.new("Frame")
mainFrame.Size = UDim2.fromScale(1, 1)
mainFrame.BackgroundColor3 = Color3.fromRGB(35, 35, 38)
mainFrame.Parent = widget

local listLayout = Instance.new("UIListLayout")
listLayout.SortOrder = Enum.SortOrder.LayoutOrder
listLayout.Padding = UDim.new(0, 6)
listLayout.HorizontalAlignment = Enum.HorizontalAlignment.Center
listLayout.VerticalAlignment = Enum.VerticalAlignment.Center
listLayout.Parent = mainFrame

local statusLabel = Instance.new("TextLabel")
statusLabel.Name = "1_StatusLabel"
statusLabel.LayoutOrder = 1
statusLabel.Size = UDim2.new(0.9, 0, 0, 26)
statusLabel.BackgroundTransparency = 1
statusLabel.TextColor3 = if token then Color3.fromRGB(80, 220, 100) else Color3.fromRGB(240, 180, 50)
statusLabel.Text = if token then "Status: Paired" else "Status: Unpaired (Enter Code)"
statusLabel.Font = Enum.Font.SourceSansBold
statusLabel.TextSize = 15
statusLabel.Parent = mainFrame

local urlInput = Instance.new("TextBox")
urlInput.Name = "2_UrlInput"
urlInput.LayoutOrder = 2
urlInput.Size = UDim2.new(0.9, 0, 0, 30)
urlInput.BackgroundColor3 = Color3.fromRGB(50, 50, 55)
urlInput.TextColor3 = Color3.fromRGB(255, 255, 255)
urlInput.PlaceholderText = "Bridge URL (e.g. http://192.168.1.129:58741)"
urlInput.Text = bridgeUrl
urlInput.ClearTextOnFocus = false
urlInput.Font = Enum.Font.SourceSans
urlInput.TextSize = 13
urlInput.Parent = mainFrame

local codeInput = Instance.new("TextBox")
codeInput.Name = "3_CodeInput"
codeInput.LayoutOrder = 3
codeInput.Size = UDim2.new(0.9, 0, 0, 30)
codeInput.BackgroundColor3 = Color3.fromRGB(50, 50, 55)
codeInput.TextColor3 = Color3.fromRGB(255, 255, 255)
codeInput.PlaceholderText = "Pairing Code (e.g. RBX-1234)"
codeInput.Text = ""
codeInput.ClearTextOnFocus = false
codeInput.Font = Enum.Font.SourceSans
codeInput.TextSize = 14
codeInput.Parent = mainFrame

local pairBtn = Instance.new("TextButton")
pairBtn.Name = "4_PairButton"
pairBtn.LayoutOrder = 4
pairBtn.Size = UDim2.new(0.9, 0, 0, 32)
pairBtn.BackgroundColor3 = Color3.fromRGB(0, 120, 215)
pairBtn.TextColor3 = Color3.fromRGB(255, 255, 255)
pairBtn.Text = "Pair / Connect"
pairBtn.Font = Enum.Font.SourceSansBold
pairBtn.TextSize = 14
pairBtn.Parent = mainFrame

toggleBtn.Click:Connect(function() widget.Enabled = not widget.Enabled end)

local startPolling: () -> ()

pairBtn.MouseButton1Click:Connect(function()
	local newUrl = normalizeUrl(urlInput.Text)
	bridgeUrl = newUrl
	plugin:SetSetting(SETTING_URL_KEY, bridgeUrl)
	urlInput.Text = bridgeUrl

	local code = codeInput.Text:gsub("%s+", "")
	if #code == 0 then
		if token then
			statusLabel.Text = "Connecting..."
			task.spawn(function()
				local success, response = pcall(function()
					return HttpService:RequestAsync({
						Url = bridgeUrl .. "/status",
						Method = "GET",
						Headers = {
							["Authorization"] = "Bearer " .. token,
						},
					})
				end)
				if success and response.Success and response.StatusCode == 200 then
					statusLabel.Text = "Status: Connected"
					statusLabel.TextColor3 = Color3.fromRGB(80, 220, 100)
					startPolling()
					return
				end
				statusLabel.Text = "Connection failed. Enter Pairing Code."
				statusLabel.TextColor3 = Color3.fromRGB(255, 80, 80)
			end)
		else
			statusLabel.Text = "Enter Pairing Code"
			statusLabel.TextColor3 = Color3.fromRGB(240, 180, 50)
		end
		return
	end

	statusLabel.Text = "Pairing..."

	task.spawn(function()
		local success, response = pcall(function()
			return HttpService:RequestAsync({
				Url = bridgeUrl .. "/pair",
				Method = "POST",
				Headers = { ["Content-Type"] = "application/json" },
				Body = HttpService:JSONEncode({ code = code }),
			})
		end)

		if success and response.Success and response.StatusCode == 200 then
			local data = HttpService:JSONDecode(response.Body)
			if data.ok and data.token then
				token = data.token
				plugin:SetSetting(SETTING_TOKEN_KEY, token)
				statusLabel.Text = "Status: Paired and Connected"
				statusLabel.TextColor3 = Color3.fromRGB(80, 220, 100)
				codeInput.Text = ""
				startPolling()
				return
			end
		end

		statusLabel.Text = "Pairing failed: Invalid code or unreachable"
		statusLabel.TextColor3 = Color3.fromRGB(255, 80, 80)
	end)
end)

LogService.MessageOut:Connect(function(message, messageType)
	local level = "output"
	if messageType == Enum.MessageType.MessageWarning then level = "warning"
	elseif messageType == Enum.MessageType.MessageError then level = "error"
	elseif messageType == Enum.MessageType.MessageInfo then level = "info" end

	table.insert(pendingLogs, {
		ts = os.time() * 1000,
		level = level,
		source = "edit",
		message = message,
	})
end)

task.spawn(function()
	while true do
		task.wait(1)
		if token and #pendingLogs > 0 then
			local logsToSend = pendingLogs
			pendingLogs = {}
			pcall(function()
				HttpService:RequestAsync({
					Url = bridgeUrl .. "/logs",
					Method = "POST",
					Headers = {
						["Content-Type"] = "application/json",
						["Authorization"] = "Bearer " .. token,
					},
					Body = HttpService:JSONEncode(logsToSend),
				})
			end)
		end
	end
end)

startPolling = function()
	if isPolling or not token then return end
	isPolling = true

	task.spawn(function()
		while isPolling and token do
			local success, response = pcall(function()
				return HttpService:RequestAsync({
					Url = bridgeUrl .. "/poll",
					Method = "GET",
					Headers = {
						["Authorization"] = "Bearer " .. token,
						["X-Session-ID"] = SESSION_ID,
						["X-DataModel-Role"] = "edit",
					},
				})
			end)

			if success and response.Success then
				if response.StatusCode == 200 and #response.Body > 0 then
					local okJson, job = pcall(HttpService.JSONDecode, HttpService, response.Body)
					if okJson and job and job.id then
						local recordingId = ChangeHistoryService:TryBeginRecording("MCP: " .. tostring(job.tool))
						local jobOk, jobVal, jobErr = executeJob(job)
						if recordingId then
							ChangeHistoryService:FinishRecording(recordingId, Enum.FinishRecordingOperation.Commit)
						end

						pcall(function()
							HttpService:RequestAsync({
								Url = bridgeUrl .. "/result",
								Method = "POST",
								Headers = {
									["Content-Type"] = "application/json",
									["Authorization"] = "Bearer " .. token,
								},
								Body = HttpService:JSONEncode({
									job_id = job.id,
									ok = jobOk,
									value = jobVal,
									error = jobErr,
								}),
							})
						end)
					end
				end
			else
				task.wait(2)
			end
			task.wait(0.05)
		end
		isPolling = false
	end)
end

if token then
	startPolling()
end
