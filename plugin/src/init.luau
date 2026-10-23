--!strict
-- Roblox Studio MCP Companion Plugin
-- Complete two-layer tool executor and HTTP bridge connector

local RunService = game:GetService("RunService")
local HttpService = game:GetService("HttpService")
local LogService = game:GetService("LogService")
local CollectionService = game:GetService("CollectionService")

-- Studio loads the plugin into every DataModel: Edit always, plus the server and client during a playtest
local ROLE = if RunService:IsEdit() then "edit" elseif RunService:IsServer() then "server" else "client"

local DEFAULT_BRIDGE_URL = "http://127.0.0.1:58741"
local SETTING_TOKEN_KEY = "ROBLOX_STUDIO_MCP_TOKEN"
local SETTING_URL_KEY = "ROBLOX_STUDIO_MCP_URL"
local SESSION_ID = HttpService:GenerateGUID(false):sub(1, 8)
local SCRIPT_CLASSES = { Script = true, LocalScript = true, ModuleScript = true }
local JOINT_CLASSES = { Weld = "WeldConstraint", AlignPosition = "AlignPosition", AlignOrientation = "AlignOrientation" }
local JOINT_PROPERTY_ALIASES = { actuator = "ActuatorType", speed = "Speed", lower = "LowerLimit", upper = "UpperLimit" }

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

local function decodeCFrame(val: any): CFrame
	if val.v and #val.v == 12 then
		return CFrame.new(table.unpack(val.v))
	end
	local pos = val.pos or {}
	local cf = CFrame.new(pos[1] or 0, pos[2] or 0, pos[3] or 0)
	if not val.rot_deg then
		return cf
	end
	local rot = val.rot_deg
	return cf * CFrame.Angles(math.rad(rot[1] or 0), math.rad(rot[2] or 0), math.rad(rot[3] or 0))
end

local function decodeEnum(name: any): EnumItem
	local parts = string.split(tostring(name), ".")
	local ok, item = pcall(function()
		return (Enum :: any)[parts[1]][parts[2]]
	end)
	if not ok or #parts ~= 2 or item == nil then
		error("unknown enum: " .. tostring(name), 0)
	end
	return item
end

local function decodeColorSequence(points: any): ColorSequence
	local keypoints = {}
	for _, kp in ipairs(points) do
		local rgb = kp.rgb or {}
		table.insert(keypoints, ColorSequenceKeypoint.new(kp.t or 0, Color3.fromRGB(rgb[1] or 255, rgb[2] or 255, rgb[3] or 255)))
	end
	if #keypoints == 1 then
		table.insert(keypoints, ColorSequenceKeypoint.new(1, keypoints[1].Value))
	end
	return ColorSequence.new(keypoints)
end

local function decodeNumberSequence(points: any): NumberSequence
	local keypoints = {}
	for _, kp in ipairs(points) do
		table.insert(keypoints, NumberSequenceKeypoint.new(kp.t or 0, kp.v or 0, kp.env or 0))
	end
	if #keypoints == 1 then
		table.insert(keypoints, NumberSequenceKeypoint.new(1, keypoints[1].Value))
	end
	return NumberSequence.new(keypoints)
end

local function decodeTagged(val: any): any
	local t = val["$t"]
	local v = if typeof(val.v) == "table" then val.v else {}
	if t == "Vector3" then return Vector3.new(v[1] or 0, v[2] or 0, v[3] or 0) end
	if t == "Vector2" then return Vector2.new(v[1] or 0, v[2] or 0) end
	if t == "UDim" then return UDim.new(v[1] or 0, v[2] or 0) end
	if t == "UDim2" then return UDim2.new(v[1] or 0, v[2] or 0, v[3] or 0, v[4] or 0) end
	if t == "Color3" then
		local rgb = val.rgb or {}
		return Color3.fromRGB(rgb[1] or 0, rgb[2] or 0, rgb[3] or 0)
	end
	if t == "CFrame" then return decodeCFrame(val) end
	if t == "Enum" then return decodeEnum(val.v) end
	if t == "Instance" then
		local inst = resolveInstance(val.ref)
		if not inst then
			error("instance not found: " .. tostring(val.ref), 0)
		end
		return inst
	end
	if t == "NumberRange" then return NumberRange.new(v[1] or 0, v[2] or v[1] or 0) end
	if t == "Rect" then return Rect.new(v[1] or 0, v[2] or 0, v[3] or 0, v[4] or 0) end
	if t == "BrickColor" then return BrickColor.new(val.v) end
	if t == "ColorSequence" then return decodeColorSequence(v) end
	if t == "NumberSequence" then return decodeNumberSequence(v) end
	error("unknown wire type: " .. tostring(t), 0)
end

local function decodeShorthand(val: { any }, targetType: string?): any
	local n = #val
	if targetType == "Vector3" and n == 3 then return Vector3.new(val[1], val[2], val[3]) end
	if targetType == "Vector2" and n == 2 then return Vector2.new(val[1], val[2]) end
	if targetType == "UDim" and n == 2 then return UDim.new(val[1], val[2]) end
	if targetType == "UDim2" and n == 4 then return UDim2.new(val[1], val[2], val[3], val[4]) end
	if targetType == "Color3" and n == 3 then return Color3.fromRGB(val[1], val[2], val[3]) end
	if targetType == "CFrame" and n == 3 then return CFrame.new(val[1], val[2], val[3]) end
	if targetType == "CFrame" and n == 12 then return CFrame.new(table.unpack(val)) end
	if targetType == "NumberRange" and n == 2 then return NumberRange.new(val[1], val[2]) end
	return nil
end

local function decode(val: any, targetType: string?): any
	if typeof(val) == "string" and targetType == "BrickColor" then
		return BrickColor.new(val)
	end
	if typeof(val) ~= "table" then
		return val
	end
	if val["$t"] then
		return decodeTagged(val)
	end
	local shorthand = decodeShorthand(val, targetType)
	if shorthand ~= nil then
		return shorthand
	end
	return val
end

local function deserialize(val: any, targetType: string?): (boolean, any, string?)
	local ok, result = pcall(decode, val, targetType)
	if not ok then
		return false, nil, tostring(result)
	end
	return true, result, nil
end

local function color3ToRGB(c: Color3): { number }
	return { math.round(c.R * 255), math.round(c.G * 255), math.round(c.B * 255) }
end

local serialize: (any, number?) -> any

local function serializeTable(tbl: { [any]: any }, depth: number): any
	if depth > 16 then
		return "<nested too deep>"
	end
	local count = 0
	for _ in pairs(tbl) do
		count += 1
	end
	local isArray = count == #tbl
	local out = {}
	for k, v in pairs(tbl) do
		out[if isArray then k else tostring(k)] = serialize(v, depth)
	end
	return out
end

serialize = function(val: any, depth: number?): any
	local t = typeof(val)
	if val == nil or t == "number" or t == "string" or t == "boolean" then
		return val
	end
	if t == "Vector3" then return { ["$t"] = "Vector3", v = { val.X, val.Y, val.Z } } end
	if t == "Vector2" then return { ["$t"] = "Vector2", v = { val.X, val.Y } } end
	if t == "UDim" then return { ["$t"] = "UDim", v = { val.Scale, val.Offset } } end
	if t == "UDim2" then return { ["$t"] = "UDim2", v = { val.X.Scale, val.X.Offset, val.Y.Scale, val.Y.Offset } } end
	if t == "Color3" then return { ["$t"] = "Color3", rgb = color3ToRGB(val) } end
	if t == "CFrame" then return { ["$t"] = "CFrame", v = { val:GetComponents() } } end
	if t == "EnumItem" then return { ["$t"] = "Enum", v = tostring(val.EnumType) .. "." .. val.Name } end
	if t == "Instance" then return { ["$t"] = "Instance", ref = val:GetDebugId(), path = val:GetFullName() } end
	if t == "BrickColor" then return { ["$t"] = "BrickColor", v = val.Name } end
	if t == "NumberRange" then return { ["$t"] = "NumberRange", v = { val.Min, val.Max } } end
	if t == "Rect" then return { ["$t"] = "Rect", v = { val.Min.X, val.Min.Y, val.Max.X, val.Max.Y } } end
	if t == "ColorSequence" then
		local points = {}
		for _, kp in ipairs(val.Keypoints) do
			table.insert(points, { t = kp.Time, rgb = color3ToRGB(kp.Value) })
		end
		return { ["$t"] = "ColorSequence", v = points }
	end
	if t == "NumberSequence" then
		local points = {}
		for _, kp in ipairs(val.Keypoints) do
			table.insert(points, { t = kp.Time, v = kp.Value, env = kp.Envelope })
		end
		return { ["$t"] = "NumberSequence", v = points }
	end
	if t == "table" then
		return serializeTable(val, (depth or 0) + 1)
	end
	return tostring(val)
end

--------------------------------------------------------------------------------
-- Instance Helpers
--------------------------------------------------------------------------------
local function fail(code: string, message: string): (boolean, any, any)
	return false, nil, { code = code, message = message }
end

local function refOf(inst: Instance): any
	return { id = inst:GetDebugId(), path = inst:GetFullName() }
end

local function describe(inst: Instance): any
	return { id = inst:GetDebugId(), path = inst:GetFullName(), class = inst.ClassName }
end

-- Reading the current value first tells the decoder which engine type a bare array should become.
local function setProperty(inst: Instance, prop: string, rawVal: any): string?
	local okRead, current = pcall(function()
		return (inst :: any)[prop]
	end)
	local okDecode, val, decodeErr = deserialize(rawVal, if okRead then typeof(current) else nil)
	if not okDecode then
		return decodeErr
	end
	local okSet, setErr = pcall(function()
		(inst :: any)[prop] = val
	end)
	if not okSet then
		return tostring(setErr)
	end
	return nil
end

local function setAttribute(inst: Instance, name: string, rawVal: any): string?
	local okDecode, val, decodeErr = deserialize(rawVal, nil)
	if not okDecode then
		return decodeErr
	end
	local okSet, setErr = pcall(inst.SetAttribute, inst, name, val)
	if not okSet then
		return tostring(setErr)
	end
	return nil
end

local function applyProperties(inst: Instance, properties: any, attributes: any, failed: { [string]: string }, prefix: string): { string }
	local applied = {}
	for prop, rawVal in pairs(properties or {}) do
		local err = setProperty(inst, prop, rawVal)
		if err then
			failed[prefix .. prop] = err
			continue
		end
		table.insert(applied, prop)
	end
	for name, rawVal in pairs(attributes or {}) do
		local err = setAttribute(inst, name, rawVal)
		if err then
			failed[prefix .. "@" .. name] = err
			continue
		end
		table.insert(applied, "@" .. name)
	end
	return applied
end

local readablePropertyCache: { [string]: { string } } = {}

local function readablePropertyNames(className: string): { string }?
	local cached = readablePropertyCache[className]
	if cached then
		return cached
	end
	local ok, props = pcall(function()
		return (game:GetService("ReflectionService") :: any):GetPropertiesOfClass(className)
	end)
	if not ok then
		return nil
	end
	local names = {}
	for _, prop in ipairs(props) do
		if prop.Permits and prop.Permits.Read ~= nil then
			table.insert(names, prop.Name)
		end
	end
	table.sort(names)
	readablePropertyCache[className] = names
	return names
end

local function readProperties(inst: Instance, spec: any): { [string]: any }
	local names = if spec == true then readablePropertyNames(inst.ClassName) or {} elseif typeof(spec) == "string" then { spec } else spec
	local out = {}
	for _, name in ipairs(names) do
		local ok, value = pcall(function()
			return (inst :: any)[name]
		end)
		if ok then
			out[name] = serialize(value)
		end
	end
	return out
end

--------------------------------------------------------------------------------
-- Recursive Tree Builder (build_tree)
--------------------------------------------------------------------------------
local function buildNode(parent: Instance, node: any, createdMap: { [string]: any }, failed: { [string]: string }): Instance
	local className = node.class or "Part"
	local inst = Instance.new(className)
	if node.name then
		inst.Name = node.name
	end

	applyProperties(inst, node.props, node.attributes, failed, (node.name or className) .. ".")

	for _, tag in ipairs(node.tags or {}) do
		CollectionService:AddTag(inst, tag)
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
		createdMap[node.name] = refOf(inst)
	end

	for _, childNode in ipairs(node.children or {}) do
		buildNode(inst, childNode, createdMap, failed)
	end

	return inst
end

--------------------------------------------------------------------------------
-- Script Helpers (manage_script)
--------------------------------------------------------------------------------
local function lastSegment(ref: any): string
	local raw = if typeof(ref) == "table" then tostring(ref[#ref]) else tostring(ref):match("([^.]+)$")
	if not raw then
		return "NewScript"
	end
	return raw:match("^(.-)#%d+$") or raw
end

local function parentRefOf(ref: any): any
	if typeof(ref) == "table" and #ref > 1 then
		return table.move(ref, 1, #ref - 1, 1, {})
	end
	if typeof(ref) == "string" and ref:sub(1, 5) == "game." then
		return ref:match("^(game%..+)%.[^.]+$")
	end
	return nil
end

local function writeSource(target: Instance, source: string): string?
	local okUpdate, updateErr = pcall(function()
		game:GetService("ScriptEditorService"):UpdateSourceAsync(target :: any, function()
			return source
		end)
	end)
	if okUpdate then
		return nil
	end
	local okSet = pcall(function()
		(target :: any).Source = source
	end)
	if okSet then
		return nil
	end
	return tostring(updateErr)
end

local scriptActions: { [string]: (any) -> (boolean, any, any) } = {}

scriptActions.read = function(args)
	local inst = resolveInstance(args.path)
	if not (inst and inst:IsA("LuaSourceContainer")) then
		return fail("STALE_REF", "script not found")
	end
	return true, { id = inst:GetDebugId(), path = inst:GetFullName(), source = (inst :: any).Source }, nil
end

scriptActions.write = function(args)
	local inst = resolveInstance(args.path)
	if inst and not inst:IsA("LuaSourceContainer") then
		return fail("COERCION", "path resolves to a " .. inst.ClassName .. ", not a script")
	end

	local created = false
	if not inst then
		local scriptType = args.script_type or "Script"
		if not SCRIPT_CLASSES[scriptType] then
			return fail("COERCION", "script_type must be Script, LocalScript or ModuleScript")
		end
		local parentRef = args.parent or parentRefOf(args.path)
		local parent = if parentRef then resolveInstance(parentRef) else workspace
		if not parent then
			return fail("STALE_REF", "parent instance not found")
		end
		local newScript = Instance.new(scriptType)
		newScript.Name = lastSegment(args.path)
		newScript.Parent = parent
		inst = newScript
		created = true
	end

	local target = inst :: Instance
	local failed = {}
	if args.run_context then
		local err = setProperty(target, "RunContext", args.run_context)
		if err then
			failed.RunContext = err
		end
	end

	local writeErr = writeSource(target, args.content or "")
	if writeErr then
		return fail("LUAU_ERROR", "failed to write source: " .. writeErr)
	end
	return true, { id = target:GetDebugId(), path = target:GetFullName(), created = created, failed = failed }, nil
end

scriptActions.delete = function(args)
	local inst = resolveInstance(args.path)
	if not (inst and inst:IsA("LuaSourceContainer")) then
		return fail("STALE_REF", "script not found")
	end
	local ref = refOf(inst)
	inst:Destroy()
	return true, { deleted = ref }, nil
end

--------------------------------------------------------------------------------
-- Job Router Implementation
--------------------------------------------------------------------------------
local handlers: { [string]: (any) -> (boolean, any, any) } = {}

handlers.execute_luau = function(args)
	if not args.code then
		return fail("COERCION", "missing 'code' argument")
	end
	local fn, compileErr = loadstring(args.code)
	if not fn then
		return fail("LUAU_ERROR", "compile error: " .. tostring(compileErr))
	end
	local ok, result = pcall(fn)
	if not ok then
		return fail("LUAU_ERROR", tostring(result))
	end
	return true, serialize(result), nil
end

handlers.build_tree = function(args)
	local parentInst = resolveInstance(args.parent)
	if not parentInst then
		return fail("STALE_REF", "parent instance not found")
	end
	local createdNodes = {}
	local failed = {}
	local ok, err = pcall(buildNode, parentInst, args.tree, createdNodes, failed)
	if not ok then
		return fail("LUAU_ERROR", tostring(err))
	end
	return true, { nodes = createdNodes, failed = failed }, nil
end

handlers.set_properties = function(args)
	local inst = resolveInstance(args.target)
	if not inst then
		return fail("STALE_REF", "target instance not found")
	end

	local failed = {}
	local applied = applyProperties(inst, args.properties, args.attributes, failed, "")

	local tags = args.tags or {}
	for _, tag in ipairs(tags.add or {}) do
		CollectionService:AddTag(inst, tag)
	end
	for _, tag in ipairs(tags.remove or {}) do
		CollectionService:RemoveTag(inst, tag)
	end

	return true, { applied = applied, failed = failed }, nil
end

handlers.get_instance_tree = function(args)
	local root = resolveInstance(args.target)
	if not root then
		return fail("STALE_REF", "target instance not found")
	end

	local includeProps = args.include_properties
	if includeProps == true and not readablePropertyNames(root.ClassName) then
		return fail("COERCION", "include_properties=true needs ReflectionService; pass an array of property names instead")
	end

	local depth = args.depth or 1
	local maxNodes = args.max_nodes or (if includeProps then 50 else 500)
	local cursor = tonumber(args.cursor) or 0
	local index = 0
	local emitted = 0
	local nextCursor: number? = nil

	-- Pre-order index is the cursor: nodes at or below it were returned earlier and come back only
	-- as `repeated` ancestors that hold newer nodes.
	local function readTree(inst: Instance, level: number): any
		if nextCursor then
			return nil
		end
		index += 1
		local isNew = index > cursor
		if isNew and emitted >= maxNodes then
			nextCursor = index - 1
			return nil
		end

		local children = inst:GetChildren()
		local node: any = {
			id = inst:GetDebugId(),
			name = inst.Name,
			class = inst.ClassName,
			path = inst:GetFullName(),
			child_count = #children,
		}
		if isNew then
			emitted += 1
			if includeProps then
				node.properties = readProperties(inst, includeProps)
			end
		else
			node.repeated = true
		end

		if level < depth then
			local childNodes = {}
			for _, child in ipairs(children) do
				local childNode = readTree(child, level + 1)
				if childNode then
					table.insert(childNodes, childNode)
				end
			end
			node.children = childNodes
		end

		if not isNew and (node.children == nil or #node.children == 0) then
			return nil
		end
		return node
	end

	local tree = readTree(root, 0)
	return true, {
		nodes = tree,
		truncated = nextCursor ~= nil,
		next_cursor = if nextCursor then tostring(nextCursor) else nil,
	}, nil
end

local function matchesFilters(desc: Instance, args: any, wantValue: any): boolean
	if args.class_name and not desc:IsA(args.class_name) then
		return false
	end
	if args.tag and not CollectionService:HasTag(desc, args.tag) then
		return false
	end
	if args.name_pattern and not string.match(desc.Name, args.name_pattern) then
		return false
	end
	if not args.attribute then
		return true
	end
	local value = desc:GetAttribute(args.attribute)
	if value == nil then
		return false
	end
	return wantValue == nil or value == wantValue
end

handlers.find_instances = function(args)
	local root = if args.root then resolveInstance(args.root) else game
	if not root then
		return fail("STALE_REF", "root instance not found")
	end
	if args.name_pattern and not pcall(string.match, "", args.name_pattern) then
		return fail("COERCION", "invalid name_pattern: " .. tostring(args.name_pattern))
	end
	if args.class_name and not pcall(game.IsA, game, args.class_name) then
		return fail("COERCION", "invalid class_name: " .. tostring(args.class_name))
	end
	if args.attribute and not pcall(game.GetAttribute, game, args.attribute) then
		return fail("COERCION", "invalid attribute name: " .. tostring(args.attribute))
	end

	local wantValue = nil
	if args.attribute_value ~= nil then
		local okDecode, val, decodeErr = deserialize(args.attribute_value, nil)
		if not okDecode then
			return fail("COERCION", "attribute_value: " .. tostring(decodeErr))
		end
		wantValue = val
	end

	local limit = args.limit or 500
	local matches = {}
	local truncated = false
	for _, desc in ipairs(root:GetDescendants()) do
		if not matchesFilters(desc, args, wantValue) then
			continue
		end
		if #matches >= limit then
			truncated = true
			break
		end
		table.insert(matches, describe(desc))
	end

	return true, { matches = matches, truncated = truncated }, nil
end

handlers.delete_instance = function(args)
	local inst = resolveInstance(args.target)
	if not inst then
		return fail("STALE_REF", "target instance not found")
	end
	local ref = refOf(inst)
	local ok, err = pcall(inst.Destroy, inst)
	if not ok then
		return fail("LUAU_ERROR", tostring(err))
	end
	return true, { deleted = ref }, nil
end

handlers.call_method = function(args)
	local inst = resolveInstance(args.target)
	if not inst then
		return fail("STALE_REF", "target instance not found")
	end
	local okIndex, method = pcall(function()
		return (inst :: any)[args.method]
	end)
	if not okIndex or typeof(method) ~= "function" then
		return fail("COERCION", "method not found: " .. tostring(args.method))
	end

	local rawArgs = args.args or {}
	local callArgs = {}
	for i = 1, #rawArgs do
		local okDecode, val, decodeErr = deserialize(rawArgs[i], nil)
		if not okDecode then
			return fail("COERCION", "argument " .. i .. ": " .. tostring(decodeErr))
		end
		callArgs[i] = val
	end

	local ok, result = pcall(method, inst, table.unpack(callArgs, 1, #rawArgs))
	if not ok then
		return fail("LUAU_ERROR", tostring(result))
	end
	return true, { value = serialize(result) }, nil
end

handlers.create_physics_joint = function(args)
	local partA = resolveInstance(args.part_a)
	local partB = resolveInstance(args.part_b)
	if not (partA and partA:IsA("BasePart") and partB and partB:IsA("BasePart")) then
		return fail("STALE_REF", "part_a or part_b is invalid BasePart")
	end

	local jointType = args.joint_type or "Weld"
	local okNew, created = pcall(Instance.new, JOINT_CLASSES[jointType] or (jointType .. "Constraint"))
	if not okNew then
		return fail("COERCION", "unknown joint_type: " .. tostring(jointType))
	end
	local joint = created :: any

	local attachments = {}
	local usesAttachments = pcall(function()
		return joint.Attachment0
	end)
	if usesAttachments then
		local attA = Instance.new("Attachment")
		attA.Name = "Attachment_" .. jointType .. "_A"
		attA.Parent = partA
		local attB = Instance.new("Attachment")
		attB.Name = "Attachment_" .. jointType .. "_B"
		attB.Parent = partB
		joint.Attachment0 = attA
		joint.Attachment1 = attB
		attachments = { refOf(attA), refOf(attB) }
	else
		joint.Part0 = partA
		joint.Part1 = partB
	end

	local props = table.clone(args.properties or {})
	local anchoredSide = props.anchored_side
	props.anchored_side = nil
	if props.lower ~= nil and props.upper ~= nil then
		props.LimitsEnabled = true
	end

	local aliases = table.clone(JOINT_PROPERTY_ALIASES)
	if pcall(function() return joint.UpperAngle end) then
		aliases.lower = "LowerAngle"
		aliases.upper = "UpperAngle"
	end

	local failed = {}
	for key, rawVal in pairs(props) do
		local name = aliases[key] or key
		local err = setProperty(joint, name, rawVal)
		if err then
			failed[name] = err
		end
	end

	joint.Parent = partA

	if anchoredSide == "a" then
		partA.Anchored = true
		partB.Anchored = false
	elseif anchoredSide == "b" then
		partB.Anchored = true
		partA.Anchored = false
	end

	return true, { joint = refOf(joint), attachments = attachments, failed = failed }, nil
end

handlers.manage_script = function(args)
	local action = args.action or "read"
	local run = scriptActions[action]
	if not run then
		return fail("COERCION", "unsupported manage_script action: " .. tostring(action))
	end
	return run(args)
end

handlers.insert_asset = function(args)
	local assetId = tonumber(args.asset_id)
	local parent = if args.parent then resolveInstance(args.parent) else workspace
	if not assetId or not parent then
		return fail("COERCION", "invalid asset_id or parent")
	end

	local ok, model = pcall(function()
		return game:GetService("InsertService"):LoadAsset(assetId)
	end)
	if not ok or not model then
		return fail("LUAU_ERROR", "failed to load asset: " .. tostring(model))
	end
	model.Parent = parent
	return true, refOf(model), nil
end

handlers.get_selection = function(_args)
	local selected = {}
	for _, inst in ipairs(game:GetService("Selection"):Get()) do
		table.insert(selected, describe(inst))
	end
	return true, { selection = selected }, nil
end

local function executeJob(job: any): (boolean, any, any)
	local handler = handlers[job.tool]
	if not handler then
		return fail("UNKNOWN", "unimplemented tool: " .. tostring(job.tool))
	end
	return handler(job.args or {})
end

local function postResult(jobId: string, ok: boolean, value: any, err: any)
	local okEncode, body = pcall(HttpService.JSONEncode, HttpService, {
		job_id = jobId,
		ok = ok,
		value = value,
		error = err,
	})
	if not okEncode then
		body = HttpService:JSONEncode({
			job_id = jobId,
			ok = false,
			error = { code = "COERCION", message = "result could not be encoded as JSON: " .. tostring(body) },
		})
	end

	pcall(function()
		HttpService:RequestAsync({
			Url = bridgeUrl .. "/result",
			Method = "POST",
			Headers = {
				["Content-Type"] = "application/json",
				["Authorization"] = "Bearer " .. (token :: string),
			},
			Body = body,
		})
	end)
end

-- A thrown error must not kill the polling loop, and a failed job is rolled back so a tool call
-- never leaves a half-applied change behind.
local function runJob(job: any)
	local history = if ROLE == "edit" then game:GetService("ChangeHistoryService") else nil
	local recordingId = if history then history:TryBeginRecording("MCP: " .. tostring(job.tool)) else nil

	local okRun, jobOk, jobVal, jobErr = pcall(executeJob, job)
	if not okRun then
		jobOk, jobVal, jobErr = false, nil, { code = "LUAU_ERROR", message = tostring(jobOk) }
	end

	if history and recordingId then
		history:FinishRecording(recordingId, if jobOk then Enum.FinishRecordingOperation.Commit else Enum.FinishRecordingOperation.Cancel)
	end

	postResult(job.id, jobOk, jobVal, jobErr)
end

local function startPolling()
	if isPolling or not token then return end
	isPolling = true

	task.spawn(function()
		while isPolling and token do
			local success, response = pcall(function()
				return HttpService:RequestAsync({
					Url = bridgeUrl .. "/poll",
					Method = "GET",
					Headers = {
						["Authorization"] = "Bearer " .. (token :: string),
						["X-Session-ID"] = SESSION_ID,
						["X-DataModel-Role"] = ROLE,
					},
				})
			end)

			if not (success and response.Success) then
				task.wait(2)
				continue
			end

			if response.StatusCode == 200 and #response.Body > 0 then
				local okJson, job = pcall(HttpService.JSONDecode, HttpService, response.Body)
				if okJson and job and job.id then
					runJob(job)
				end
			end
			task.wait(0.05)
		end
		isPolling = false
	end)
end

LogService.MessageOut:Connect(function(message, messageType)
	local level = "output"
	if messageType == Enum.MessageType.MessageWarning then level = "warning"
	elseif messageType == Enum.MessageType.MessageError then level = "error"
	elseif messageType == Enum.MessageType.MessageInfo then level = "info" end

	table.insert(pendingLogs, {
		ts = os.time() * 1000,
		level = level,
		source = ROLE,
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
						["Authorization"] = "Bearer " .. (token :: string),
					},
					Body = HttpService:JSONEncode(logsToSend),
				})
			end)
		end
	end
end)

--------------------------------------------------------------------------------
-- UI Setup (Edit DataModel only)
--------------------------------------------------------------------------------
if ROLE == "edit" then
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
	listLayout.Padding = UDim.new(0, 6)
	listLayout.HorizontalAlignment = Enum.HorizontalAlignment.Center
	listLayout.VerticalAlignment = Enum.VerticalAlignment.Center
	listLayout.Parent = mainFrame

	local statusLabel = Instance.new("TextLabel")
	statusLabel.Size = UDim2.new(0.9, 0, 0, 28)
	statusLabel.BackgroundTransparency = 1
	statusLabel.TextColor3 = if token then Color3.fromRGB(80, 220, 100) else Color3.fromRGB(240, 180, 50)
	statusLabel.Text = if token then "Status: Paired" else "Status: Unpaired (Enter Code)"
	statusLabel.Font = Enum.Font.SourceSansBold
	statusLabel.TextSize = 15
	statusLabel.Parent = mainFrame

	local urlInput = Instance.new("TextBox")
	urlInput.Size = UDim2.new(0.9, 0, 0, 30)
	urlInput.BackgroundColor3 = Color3.fromRGB(50, 50, 55)
	urlInput.TextColor3 = Color3.fromRGB(255, 255, 255)
	urlInput.PlaceholderText = "Bridge URL (e.g. http://192.168.1.50:58741)"
	urlInput.Text = bridgeUrl
	urlInput.Font = Enum.Font.SourceSans
	urlInput.TextSize = 13
	urlInput.Parent = mainFrame

	local codeInput = Instance.new("TextBox")
	codeInput.Size = UDim2.new(0.9, 0, 0, 30)
	codeInput.BackgroundColor3 = Color3.fromRGB(50, 50, 55)
	codeInput.TextColor3 = Color3.fromRGB(255, 255, 255)
	codeInput.PlaceholderText = "Pairing Code (e.g. RBX-1234)"
	codeInput.Font = Enum.Font.SourceSans
	codeInput.TextSize = 14
	codeInput.Parent = mainFrame

	local pairBtn = Instance.new("TextButton")
	pairBtn.Size = UDim2.new(0.9, 0, 0, 32)
	pairBtn.BackgroundColor3 = Color3.fromRGB(0, 120, 215)
	pairBtn.TextColor3 = Color3.fromRGB(255, 255, 255)
	pairBtn.Text = "Pair / Connect"
	pairBtn.Font = Enum.Font.SourceSansBold
	pairBtn.TextSize = 14
	pairBtn.Parent = mainFrame

	toggleBtn.Click:Connect(function() widget.Enabled = not widget.Enabled end)

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
end

if token and ROLE ~= "client" then
	startPolling()
end
