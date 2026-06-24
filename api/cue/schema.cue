package api

#Source: {
	schema_version: string
	api: {
		base_path: string
	}
	info: {
		title: string
		version: string
		description?: string
	}
	openapi?: _
	servers?: [..._]
	tags?: [..._]
	schemas: [string]: _
	endpoints: [..._]
}
