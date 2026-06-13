package dashboard

type Signals struct {
	Filters Filters `json:"filters"`
}

type Filters struct {
	DateRange string `json:"dateRange"`
	State     string `json:"state"`
	Category  string `json:"category"`
}

func (f Filters) WithDefaults() Filters {
	if f.DateRange == "" {
		f.DateRange = "all"
	}
	if f.State == "" {
		f.State = "all"
	}
	return f
}

type Patch struct {
	Filters Filters          `json:"filters"`
	Status  Status           `json:"status"`
	KPIs    []KPI            `json:"kpis"`
	Charts  map[string]Chart `json:"charts"`
}

type Status struct {
	Loading       bool   `json:"loading"`
	Error         string `json:"error"`
	LastUpdated   string `json:"lastUpdated"`
	DataDirectory string `json:"dataDirectory"`
	SetupRequired bool   `json:"setupRequired"`
}

type KPI struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Note  string `json:"note"`
	Tone  string `json:"tone"`
}

type Chart struct {
	Title string  `json:"title"`
	Unit  string  `json:"unit"`
	Data  []Point `json:"data"`
}

type Point struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

func EmptyPatch(filters Filters, dataDir string, err error) Patch {
	message := ""
	if err != nil {
		message = err.Error()
	}

	return Patch{
		Filters: filters.WithDefaults(),
		Status: Status{
			Loading:       false,
			Error:         message,
			DataDirectory: dataDir,
			SetupRequired: err != nil,
		},
		KPIs: []KPI{
			{Label: "Orders", Value: "-", Note: "Waiting for CSVs", Tone: "neutral"},
			{Label: "Revenue", Value: "-", Note: "Waiting for CSVs", Tone: "neutral"},
			{Label: "AOV", Value: "-", Note: "Waiting for CSVs", Tone: "neutral"},
			{Label: "Review", Value: "-", Note: "Waiting for CSVs", Tone: "neutral"},
		},
		Charts: map[string]Chart{
			"revenue":    {Title: "Revenue by month", Unit: "R$", Data: []Point{}},
			"orders":     {Title: "Orders by status", Unit: "orders", Data: []Point{}},
			"categories": {Title: "Top product categories", Unit: "R$", Data: []Point{}},
			"delivery":   {Title: "Delivery delay", Unit: "orders", Data: []Point{}},
		},
	}
}
