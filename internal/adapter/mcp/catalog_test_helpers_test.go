package mcp

func toolsByName(tools []Tool) map[string]Tool {
	byName := make(map[string]Tool, len(tools))
	for _, tool := range tools {
		byName[tool.Name] = tool
	}
	return byName
}

func schemaHasProperty(schema map[string]any, name string) bool {
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return false
	}
	_, ok = properties[name]
	return ok
}

func schemaRequires(schema map[string]any, name string) bool {
	required, ok := schema["required"].([]string)
	if !ok {
		return false
	}
	for _, item := range required {
		if item == name {
			return true
		}
	}
	return false
}

func schemaPropertyDescription(schema map[string]any, name string) string {
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return ""
	}
	property, ok := properties[name].(map[string]any)
	if !ok {
		return ""
	}
	description, _ := property["description"].(string)
	return description
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
