--!strict
-- Roblox Studio MCP Type Deserializer and Serializer
-- Maps between JSON wire format and native Luau engine data types

local Deserializer = {}

local function resolveInstance(ref: any): Instance?
	if typeof(ref) == "string" then
		-- Check if it is a dotted path
		if ref:sub(1, 5) == "game." then
			local segments = ref:split(".")
			local current: any = game
			for i = 2, #segments do
				local seg = segments[i]
				current = current:FindFirstChild(seg)
				if not current then
					return nil
				end
			end
			return current
		end

		-- Otherwise search by name or debug ID
		for _, desc in ipairs(game:GetDescendants()) do
			if desc.Name == ref or (desc:GetDebugId() == ref) then
				return desc
			end
		end
	elseif typeof(ref) == "table" and #ref > 0 then
		-- Segment array: ["Workspace", "Map", "Stage#2"]
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
			if not current then
				return nil
			end
		end
		return current
	end
	return nil
end

Deserializer.ResolveInstance = resolveInstance

-- Deserializes a wire-format JSON value into a native Luau value
function Deserializer.Deserialize(val: any, targetPropType: string?): (boolean, any, string?)
	if val == nil then
		return true, nil, nil
	end

	local valType = typeof(val)
	if valType == "number" or valType == "string" or valType == "boolean" then
		return true, val, nil
	end

	if valType == "table" then
		-- Check for tagged wire format: {"$t": "Type", ...}
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
					if enumGroup then
						local item = enumGroup[parts[2]]
						if item then
							return true, item, nil
						end
					end
				end
				return false, nil, "unknown enum: " .. tostring(val.v)
			elseif t == "Instance" and val.ref then
				local inst = resolveInstance(val.ref)
				if inst then
					return true, inst, nil
				end
				return false, nil, "failed to resolve instance: " .. tostring(val.ref)
			elseif t == "NumberRange" and val.v then
				return true, NumberRange.new(val.v[1] or 0, val.v[2] or val.v[1] or 0), nil
			elseif t == "Rect" and val.v then
				return true, Rect.new(val.v[1] or 0, val.v[2] or 0, val.v[3] or 0, val.v[4] or 0), nil
			elseif t == "BrickColor" and val.v then
				return true, BrickColor.new(val.v), nil
			elseif t == "ColorSequence" and val.v then
				local keypoints = {}
				for _, kp in ipairs(val.v) do
					local c = Color3.fromRGB(kp.rgb[1] or 255, kp.rgb[2] or 255, kp.rgb[3] or 255)
					table.insert(keypoints, ColorSequenceKeypoint.new(kp.t or 0, c))
				end
				if #keypoints == 1 then
					table.insert(keypoints, ColorSequenceKeypoint.new(1, keypoints[1].Value))
				end
				return true, ColorSequence.new(keypoints), nil
			elseif t == "NumberSequence" and val.v then
				local keypoints = {}
				for _, kp in ipairs(val.v) do
					table.insert(keypoints, NumberSequenceKeypoint.new(kp.t or 0, kp.v or 0, kp.env or 0))
				end
				if #keypoints == 1 then
					table.insert(keypoints, NumberSequenceKeypoint.new(1, keypoints[1].Value))
				end
				return true, NumberSequence.new(keypoints), nil
			end
		end

		-- Coercion shorthand when target property type is known
		if targetPropType == "Vector3" and #val == 3 then
			return true, Vector3.new(val[1], val[2], val[3]), nil
		elseif targetPropType == "Vector2" and #val == 2 then
			return true, Vector2.new(val[1], val[2]), nil
		elseif targetPropType == "UDim2" and #val == 4 then
			return true, UDim2.new(val[1], val[2], val[3], val[4]), nil
		elseif targetPropType == "Color3" and #val == 3 then
			return true, Color3.fromRGB(val[1], val[2], val[3]), nil
		end
	end

	return true, val, nil
end

-- Serializes a native Luau value into the wire-format JSON representation
function Deserializer.Serialize(val: any): any
	if val == nil then
		return nil
	end

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

return Deserializer
