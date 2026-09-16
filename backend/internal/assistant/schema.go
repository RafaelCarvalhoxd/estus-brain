package assistant

// Small builders for the JSON Schemas tools declare. Descriptions are in
// Portuguese: they're read by models answering a Portuguese speaker.

func object(props map[string]any, required ...string) Schema {
	if required == nil {
		required = []string{}
	}
	return Schema{"type": "object", "properties": props, "required": required, "additionalProperties": false}
}

func str(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func enum(desc string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": desc, "enum": values}
}

func number(desc string) map[string]any {
	return map[string]any{"type": "number", "description": desc}
}

func integer(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}

func boolean(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}

func array(desc string, items map[string]any) map[string]any {
	return map[string]any{"type": "array", "description": desc, "items": items}
}
