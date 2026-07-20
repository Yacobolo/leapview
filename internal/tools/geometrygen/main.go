// Command geometrygen vendors the pinned, authoritative IBGE state geometry
// used by the first-party geographic showcase.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

const sourceURL = "https://servicodados.ibge.gov.br/api/v3/malhas/paises/BR?formato=application%2Fvnd.geo%2Bjson&qualidade=minima&intrarregiao=UF"

var stateCodes = map[string]string{
	"11":"RO", "12":"AC", "13":"AM", "14":"RR", "15":"PA", "16":"AP", "17":"TO",
	"21":"MA", "22":"PI", "23":"CE", "24":"RN", "25":"PB", "26":"PE", "27":"AL", "28":"SE", "29":"BA",
	"31":"MG", "32":"ES", "33":"RJ", "35":"SP", "41":"PR", "42":"SC", "43":"RS",
	"50":"MS", "51":"MT", "52":"GO", "53":"DF",
}

type featureCollection struct {
	Type string `json:"type"`
	Features []feature `json:"features"`
	Metadata metadata `json:"libredash"`
}
type feature struct {
	Type string `json:"type"`
	ID string `json:"id"`
	Geometry json.RawMessage `json:"geometry"`
	Properties map[string]any `json:"properties"`
}
type metadata struct {
	Source string `json:"source"`
	Attribution string `json:"attribution"`
	License string `json:"license"`
	IdentifierSystem string `json:"identifierSystem"`
}

func main() {
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Get(sourceURL)
	if err != nil { panic(err) }
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK { panic(fmt.Errorf("IBGE geometry returned %s", response.Status)) }
	var collection featureCollection
	decoder := json.NewDecoder(response.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&collection); err != nil { panic(err) }
	if collection.Type != "FeatureCollection" || len(collection.Features) != 27 { panic(fmt.Errorf("unexpected IBGE state collection with %d features", len(collection.Features))) }
	for index := range collection.Features {
		code, _ := collection.Features[index].Properties["codarea"].(string)
		abbreviation := stateCodes[code]
		if abbreviation == "" { panic(fmt.Errorf("unknown IBGE state code %q", code)) }
		collection.Features[index].ID = abbreviation
		collection.Features[index].Properties["id"] = abbreviation
	}
	collection.Metadata = metadata{Source: sourceURL, Attribution: "Instituto Brasileiro de Geografia e Estatística (IBGE)", License: "IBGE data reuse terms", IdentifierSystem: "br-uf"}
	data, err := json.Marshal(collection)
	if err != nil { panic(err) }
	data = append(data, '\n')
	if err := os.WriteFile("static/geometry/br-states-ibge.geojson", data, 0o644); err != nil { panic(err) }
}
