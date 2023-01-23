package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Entity struct {
	uri             string
	lcLabel         string
	prefLabel       string
	definition      string
	lcSynonyms      []string
	synonyms        []string
	instances       []string
	encodes         []string
	fromScore       int
	toScore         int
	refScore        int
	annotationScore float64
	entityType      string
	pubMeds         []string
}

type SimpleEntity struct {
	uri        string
	lcLabel    string
	prefLabel  string
	definition string
}

type Statement struct {
	uri        string
	lcLabel    string
	prefLabel  string
	definition string
	subject    string
	object     string
	predicate  string
}

var prefLabelRT = "<http://www.w3.org/2004/02/skos/core#prefLabel>"
var labelRT = "<http://www.w3.org/2000/01/rdf-schema#label>"
var definitionRT = "<http://www.w3.org/2004/02/skos/core#definition>"
var synonymRT = "<http://www.w3.org/2004/02/skos/core#altLabel>"
var instanceRT = "<http://schema.org/evidenceOrigin>"
var evidenceRT = "<http://schema.org/evidenceLevel>"
var encodesRT = "<http://semanticscience.org/resource/SIO_010078>"
var taxonRT = "<http://purl.obolibrary.org/obo/RO_0000052>"
var typeRT = "<http://www.w3.org/1999/02/22-rdf-syntax-ns#type>"
var pubMedRT = "<http://semanticscience.org/resource/SIO_000772>"
var statementObject = "<http://www.w3.org/1999/02/22-rdf-syntax-ns#object>"
var statementPredicate = "<http://www.w3.org/1999/02/22-rdf-syntax-ns#predicate>"
var statementSubject = "<http://www.w3.org/1999/02/22-rdf-syntax-ns#subject>"

var taxonPrefix = "http://purl.obolibrary.org/obo/NCBITaxon_"

var threadCount = 10

// altLabelRT := "http://www.w3.org/2004/02/skos/core#altLabel"
var classURI = "<http://www.w3.org/2002/07/owl#Class>"

/*
var taxa = []string{
	"3055",
	"3702",
	"4577",
	"6239",
	"7227",
	"7955",
	"9031",
	"9606",
	"9615",
	"9823",
	"9913",
	"9986",
	"10090",
	"10116",
	"36329",
	"39947",
	"44689",
	"284812",
	"367110",
	"559292",
}
*/

/*
var taxa = []string{
	"9606",
}*/

var taxa = []string{}

func main() {
	if len(os.Args) < 2 {
		panic("Missing RDF folder path!")
	}
	var rdfPath string
	flag.StringVar(&rdfPath, "path", "uploads", "rdf path")
	flag.IntVar(&threadCount, "t", 10, "thread count")
	flag.Parse()

	fmt.Print("MetaDB Generator started...\n")

	mongoURI := "mongodb://localhost:27027"
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoURI))
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	refScores := make(map[string]int)

	// Gene Ontology
	// parseGeneOntology(rdfPath, client)

	// Proteins and Genes for all taxa
	for _, taxon := range taxa {
		fmt.Println("Parsing RDFs for taxon", taxon)
		parseEntityRDF(taxon, "prot", "http://rdf.biogateway.eu/prot", rdfPath, refScores, client)
		parseEntityRDF(taxon, "gene", "http://rdf.biogateway.eu/gene", rdfPath, refScores, client)

		parseStatementRefScore(taxon, "prot2bp", "http://rdf.biogateway.eu/prot-onto/", rdfPath, refScores)
		parseStatementRefScore(taxon, "prot2cc", "http://rdf.biogateway.eu/prot-onto/", rdfPath, refScores)
		parseStatementRefScore(taxon, "prot2mf", "http://rdf.biogateway.eu/prot-onto/", rdfPath, refScores)

		parseStatementRDF(taxon, "prot2prot", "http://rdf.biogateway.eu/prot-prot/uniprot!", rdfPath, client)

	}
	// We only have diseases for humans
	parseStatementRefScore("9606", "gene2phen", "http://rdf.biogateway.eu/gene-phen/", rdfPath, refScores)
	parseDiseases(rdfPath, refScores, client)

	// Depends on parsing prot2bp, prot2cc and prot2mf first, to get accurate refScores.
	parseGeneOntology(rdfPath, refScores, client)

	fmt.Printf("Done!")
}

func parseEntityRDF(taxon string, graph string, prefix string, rdfPath string, refScores map[string]int, client *mongo.Client) {
	f, err := os.Open(rdfPath + "/" + graph + "/" + taxon + ".nt")
	if err != nil {
		fmt.Print("Error opening file: ", err)
	}
	defer f.Close()
	// protDB := client.Database("metadb").Collection("prot")
	lineNumber := 0
	scanner := bufio.NewScanner(f)
	entityMap := make(map[string]Entity)

	for scanner.Scan() {
		line := scanner.Text()
		lineNumber++
		components := strings.SplitN(line, " ", 3)
		sub := components[0]
		predicate := components[1]
		value := cleanRDFString(components[2])

		uri := removeLTGT(sub)
		if !strings.HasPrefix(uri, prefix) {
			continue
		}
		if predicate == prefLabelRT {
			if entry, ok := entityMap[uri]; ok {
				entry.prefLabel = value
				entry.lcLabel = strings.ToLower(value)
				entityMap[uri] = entry
			} else {
				entityMap[uri] = Entity{
					uri:       uri,
					prefLabel: value,
					lcLabel:   strings.ToLower(value),
				}
			}
		} else if predicate == definitionRT {
			if entry, ok := entityMap[uri]; ok {
				entry.definition = value
				entityMap[uri] = entry
			} else {
				entityMap[uri] = Entity{
					uri:        uri,
					definition: value}
			}
		} else if predicate == synonymRT {
			if entry, ok := entityMap[uri]; ok {
				if len(entry.synonyms) > 0 {
					entry.synonyms = append(entry.synonyms, value)
				} else {
					entry.synonyms = []string{value}
				}
				entityMap[uri] = entry
			} else {
				entityMap[uri] = Entity{
					uri:      uri,
					synonyms: []string{value},
				}
			}
		} else if predicate == instanceRT {
			instanceURI := removeLTGT(value)
			if entry, ok := entityMap[uri]; ok {
				if len(entry.instances) > 0 {
					entry.instances = append(entry.instances, instanceURI)
				} else {
					entry.instances = []string{instanceURI}
				}
				entityMap[uri] = entry
			} else {
				entityMap[uri] = Entity{
					uri:       uri,
					instances: []string{instanceURI},
				}
			}
		} else if predicate == encodesRT {
			protUri := removeLTGT(value)
			if entry, ok := entityMap[uri]; ok {
				if len(entry.encodes) > 0 {
					entry.encodes = append(entry.encodes, protUri)
				} else {
					entry.encodes = []string{protUri}
				}
				entityMap[uri] = entry
			} else {
				entityMap[uri] = Entity{
					uri:     uri,
					encodes: []string{protUri},
				}
			}
		} else if predicate == pubMedRT {
			pubMedURI := removeLTGT(value)
			if entry, ok := entityMap[uri]; ok {
				if len(entry.pubMeds) > 0 {
					entry.pubMeds = append(entry.pubMeds, pubMedURI)
				} else {
					entry.pubMeds = []string{pubMedURI}
				}
				entityMap[uri] = entry
			} else {
				entityMap[uri] = Entity{
					uri:     uri,
					pubMeds: []string{pubMedURI},
				}
			}
		} else if predicate == evidenceRT {
			if entry, ok := entityMap[uri]; ok {
				if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
					entry.annotationScore = floatValue
					entityMap[uri] = entry
				}

			} else {
				entityMap[uri] = Entity{
					uri:        uri,
					definition: value}
			}
		} else if predicate == typeRT {
			if value == classURI {
				if entry, ok := entityMap[uri]; ok {
					entry.entityType = removeLTGT(value)
				} else {
					entityMap[uri] = Entity{
						uri:        uri,
						entityType: removeLTGT(value)}
				}
			} else {
				// This is an instance of another class.
				// TODO: Add it to the array of instances for the root class.
			}
		}

		if lineNumber%1000 == 0 {
			fmt.Printf("[%s][%s] Parsed line number %d\n", taxon, graph, lineNumber)
		}
	}

	for key, entity := range entityMap {
		refScore := 0
		if graph == "prot" {
			refScore = len(entity.pubMeds)
		}
		if graph == "gene" {
			for _, v := range entity.encodes {
				protRefScore := refScores[v]
				if protRefScore > 0 {
					refScore += protRefScore
				}
			}
		}
		refScores[key] = refScore
	}

	entitiesPerThread := (len(entityMap) / threadCount) + 1
	entities := make([][]Entity, threadCount)

	for index, _ := range entities {

		entities[index] = make([]Entity, entitiesPerThread)
		i := 0
		// Go through map of entities, put in list for each thread, and remove from entityMap.
		for key, entity := range entityMap {
			if i > entitiesPerThread-1 {
				continue
			}
			entities[index][i] = entity

			delete(entityMap, key)
			i++
		}
	}

	var waitGroup sync.WaitGroup
	waitGroup.Add(threadCount)

	for index, list := range entities {
		go func(i int, list []Entity) {
			defer waitGroup.Done()
			insertEntitiesToDB(list, client, i, graph, taxon, refScores)
		}(index, list)
	}
	waitGroup.Wait()
}

func parseStatementRDF(taxon string, graph string, prefix string, rdfPath string, client *mongo.Client) {
	f, err := os.Open(rdfPath + "/" + graph + "/" + taxon + ".nt")
	if err != nil {
		fmt.Print("Error opening file: ", err)
	}
	defer f.Close()
	// protDB := client.Database("metadb").Collection("prot")
	lineNumber := 0
	scanner := bufio.NewScanner(f)
	statementMap := make(map[string]Statement)

	for scanner.Scan() {
		line := scanner.Text()
		lineNumber++
		components := strings.SplitN(line, " ", 3)
		sub := components[0]
		predicate := components[1]
		value := cleanRDFString(components[2])

		uri := removeLTGT(sub)
		if !strings.HasPrefix(uri, prefix) {
			continue
		}
		if predicate == prefLabelRT {
			if entry, ok := statementMap[uri]; ok {
				entry.prefLabel = value
				entry.lcLabel = strings.ToLower(value)
				statementMap[uri] = entry
			} else {
				statementMap[uri] = Statement{
					uri:       uri,
					prefLabel: value,
					lcLabel:   strings.ToLower(value),
				}
			}
		} else if predicate == definitionRT {
			if entry, ok := statementMap[uri]; ok {
				entry.definition = value
				statementMap[uri] = entry
			} else {
				statementMap[uri] = Statement{
					uri:        uri,
					definition: value}
			}
		} else if predicate == statementPredicate {
			if entry, ok := statementMap[uri]; ok {
				entry.predicate = removeLTGT(value)
				statementMap[uri] = entry
			} else {
				statementMap[uri] = Statement{
					uri:       uri,
					predicate: removeLTGT(value)}
			}
		} else if predicate == statementObject {
			if entry, ok := statementMap[uri]; ok {
				entry.object = removeLTGT(value)
				statementMap[uri] = entry
			} else {
				statementMap[uri] = Statement{
					uri:    uri,
					object: removeLTGT(value)}
			}
		} else if predicate == statementSubject {
			if entry, ok := statementMap[uri]; ok {
				entry.subject = removeLTGT(value)
				statementMap[uri] = entry
			} else {
				statementMap[uri] = Statement{
					uri:     uri,
					subject: removeLTGT(value)}
			}
		}
		if lineNumber%1000 == 0 {
			fmt.Printf("[%s][%s] Parsed line number %d\n", taxon, graph, lineNumber)
		}
	}
	entitiesPerThread := (len(statementMap) / threadCount) + 1
	entities := make([][]Statement, threadCount)

	for index, _ := range entities {
		entities[index] = make([]Statement, entitiesPerThread)
		i := 0
		for key, statement := range statementMap {
			if i > entitiesPerThread-1 {
				continue
			}
			entities[index][i] = statement
			delete(statementMap, key)
			i++
		}
	}

	var waitGroup sync.WaitGroup
	waitGroup.Add(threadCount)

	for index, list := range entities {
		go func(i int, list []Statement) {
			defer waitGroup.Done()
			insertStatementsToDB(list, client, i, graph, taxon)
		}(index, list)
	}
	waitGroup.Wait()
}

func parseStatementRefScore(taxon string, graph string, prefix string, rdfPath string, refScores map[string]int) {
	f, err := os.Open(rdfPath + "/" + graph + "/" + taxon + ".nt")
	if err != nil {
		fmt.Print("Error opening file: ", err)
	}
	defer f.Close()
	// protDB := client.Database("metadb").Collection("prot")
	lineNumber := 0
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Text()
		lineNumber++
		components := strings.SplitN(line, " ", 3)
		sub := components[0]
		predicate := components[1]
		value := cleanRDFString(components[2])

		uri := removeLTGT(sub)
		if !strings.HasPrefix(uri, prefix) {
			continue
		}
		if predicate == statementObject {
			object := removeLTGT(value)
			refScores[object] += 1
		}
		if lineNumber%1000 == 0 {
			fmt.Printf("[%s][%s] Parsed line number %d\n", taxon, graph, lineNumber)
		}
	}
}

func parseGeneOntology(rdfPath string, refScores map[string]int, client *mongo.Client) {
	oboDefinitionRT := "<http://purl.obolibrary.org/obo/IAO_0000115>"

	f, err := os.Open(rdfPath + "/go/go-basic.nt")
	if err != nil {
		fmt.Print("Error opening file: ", err)
	}
	defer f.Close()
	lineNumber := 0
	scanner := bufio.NewScanner(f)
	entityMap := make(map[string]SimpleEntity)

	for scanner.Scan() {
		line := scanner.Text()
		lineNumber++
		components := strings.SplitN(line, " ", 3)
		if len(components) < 3 {
			continue
		}
		sub := components[0]
		predicate := components[1]
		value := cleanRDFString(components[2])

		uri := removeLTGT(sub)
		if !strings.HasPrefix(uri, "http://purl.obolibrary.org/obo") {
			continue
		}
		if predicate == labelRT {
			if entry, ok := entityMap[uri]; ok {
				entry.prefLabel = value
				entry.lcLabel = strings.ToLower(value)
				entityMap[uri] = entry
			} else {
				entityMap[uri] = SimpleEntity{
					uri:       uri,
					prefLabel: value,
					lcLabel:   strings.ToLower(value),
				}
			}
		} else if predicate == oboDefinitionRT {
			if entry, ok := entityMap[uri]; ok {
				entry.definition = value
				entityMap[uri] = entry
			} else {
				entityMap[uri] = SimpleEntity{
					uri:        uri,
					definition: value}
			}
		}
		if lineNumber%1000 == 0 {
			fmt.Println("[GeneOntology] Parsed line number", lineNumber)
		}
	}
	fmt.Println("Parsing complete!")
	entitiesPerThread := (len(entityMap) / threadCount) + 1
	entities := make([][]SimpleEntity, threadCount)

	for index, _ := range entities {
		entities[index] = make([]SimpleEntity, entitiesPerThread)
		i := 0
		for key, prot := range entityMap {
			if i > entitiesPerThread-1 {
				continue
			}
			entities[index][i] = prot
			delete(entityMap, key)
			i++
		}
	}

	var waitGroup sync.WaitGroup
	waitGroup.Add(threadCount)

	for index, list := range entities {
		go func(i int, list []SimpleEntity) {
			defer waitGroup.Done()
			insertSimpleEntitiesToDB(list, client, i, "goall", refScores)
		}(index, list)
	}
	waitGroup.Wait()
}

func parseDiseases(rdfPath string, refScores map[string]int, client *mongo.Client) {
	// TODO: This should not be hardcoded if 22 refers to a year.
	f, err := os.Open(rdfPath + "/omim/omim-22.nt")
	if err != nil {
		fmt.Print("Error opening file: ", err)
	}
	defer f.Close()
	lineNumber := 0
	scanner := bufio.NewScanner(f)
	entityMap := make(map[string]SimpleEntity)

	for scanner.Scan() {
		line := scanner.Text()
		lineNumber++
		components := strings.SplitN(line, " ", 3)
		if len(components) < 3 {
			continue
		}
		sub := components[0]
		predicate := components[1]
		value := cleanRDFString(components[2])

		uri := removeLTGT(sub)
		if !strings.HasPrefix(uri, "http://purl.bioontology.org/ontology/") {
			continue
		}

		if predicate == prefLabelRT {
			if entry, ok := entityMap[uri]; ok {
				entry.prefLabel = value
				entry.lcLabel = strings.ToLower(value)
				entityMap[uri] = entry
			} else {
				entityMap[uri] = SimpleEntity{
					uri:       uri,
					prefLabel: value,
					lcLabel:   strings.ToLower(value),
				}
			}
		}
		if lineNumber%1000 == 0 {
			fmt.Println("[Omim] Parsed line number", lineNumber)
		}
	}
	fmt.Println("Parsing complete!")
	entitiesPerThread := (len(entityMap) / threadCount) + 1
	entities := make([][]SimpleEntity, threadCount)

	for index, _ := range entities {
		entities[index] = make([]SimpleEntity, entitiesPerThread)
		i := 0
		for key, prot := range entityMap {
			if i > entitiesPerThread-1 {
				continue
			}
			entities[index][i] = prot
			delete(entityMap, key)
			i++
		}
	}

	var waitGroup sync.WaitGroup
	waitGroup.Add(threadCount)

	for index, list := range entities {
		go func(i int, list []SimpleEntity) {
			defer waitGroup.Done()
			insertSimpleEntitiesToDB(list, client, i, "omim", refScores)
		}(index, list)
	}
	waitGroup.Wait()
}

func removeLTGT(value string) string {
	return strings.Replace(strings.Replace(value, ">", "", 1), "<", "", 1)
}

/*
 */

func insertStatementsToDB(statements []Statement, client *mongo.Client, index int, graph string, taxon string) {
	updateOptions := options.Update().SetUpsert(true)
	// insertOptions := options.InsertOne().SetBypassDocumentValidation(true)
	collection := client.Database("metadb").Collection(graph)
	dbIndices := []mongo.IndexModel{
		{Keys: bson.M{"uri": 1}},
		{Keys: bson.M{"lcLabel": 1}},
		{Keys: bson.M{"subject": 1}},
		{Keys: bson.M{"object": 1}},
		{Keys: bson.M{"predicate": 1}},
		{Keys: bson.M{"taxon": 1}},
	}

	_, err := collection.Indexes().CreateMany(context.TODO(), dbIndices)
	if err != nil {
		panic(err)
	}
	statementNumber := 0
	timestamp := time.Now().Unix()
	for _, statement := range statements {
		statementNumber++

		doc := bson.M{
			"uri":        statement.uri,
			"prefLabel":  statement.prefLabel,
			"lcLabel":    statement.lcLabel,
			"definition": statement.definition,
			"subject":    statement.subject,
			"object":     statement.object,
			"predicate":  statement.predicate,
			"taxon":      taxonPrefix + taxon,
		}
		_, err := collection.UpdateOne(
			context.TODO(),
			bson.M{"uri": statement.uri},
			bson.M{"$set": doc},
			updateOptions)
		if err != nil {
			panic(err)
		}
		if statementNumber%1000 == 0 {
			nowTime := time.Now().Unix()
			duration := nowTime - timestamp
			timestamp = nowTime
			fmt.Println("Thread", index, "Inserted", statementNumber, "into mongoDB graph", graph, "in", duration, "seconds")
		}
	}
}

func insertSimpleEntitiesToDB(entities []SimpleEntity, client *mongo.Client, index int, graph string, refScores map[string]int) {
	updateOptions := options.Update().SetUpsert(true)
	// insertOptions := options.InsertOne().SetBypassDocumentValidation(true)
	entityDB := client.Database("metadb").Collection(graph)
	dbIndices := []mongo.IndexModel{
		{Keys: bson.M{"uri": 1}},
		{Keys: bson.M{"lcLabel": 1}},
		{Keys: bson.M{"lcSynonyms": 1}},
		{Keys: bson.M{"refScore": 1}},
		{Keys: bson.M{"definition": "text"}},
	}

	_, err := entityDB.Indexes().CreateMany(context.TODO(), dbIndices)
	if err != nil {
		panic(err)
	}
	entityNumber := 0
	timestamp := time.Now().Unix()
	for _, entity := range entities {
		entityNumber++
		refScore := refScores[entity.uri]

		doc := bson.M{
			"uri":        entity.uri,
			"prefLabel":  entity.prefLabel,
			"lcLabel":    entity.lcLabel,
			"definition": entity.definition,
			"refScore":   refScore,
			// "pubMedRefs":      entity.pubMeds,
		}
		_, err := entityDB.UpdateOne(
			context.TODO(),
			bson.M{"uri": entity.uri},
			bson.M{"$set": doc},
			updateOptions)
		if err != nil {
			panic(err)
		}
		if entityNumber%1000 == 0 {
			nowTime := time.Now().Unix()
			duration := nowTime - timestamp
			timestamp = nowTime
			fmt.Println("Thread", index, "Inserted", entityNumber, "into mongoDB graph", graph, "in", duration, "seconds")
		}
	}
}

func insertEntitiesToDB(entities []Entity, client *mongo.Client, index int, graph string, taxon string, refScores map[string]int) {
	updateOptions := options.Update().SetUpsert(true)
	// insertOptions := options.InsertOne().SetBypassDocumentValidation(true)
	entityDB := client.Database("metadb").Collection(graph)
	dbIndices := []mongo.IndexModel{
		{Keys: bson.M{"uri": 1}},
		{Keys: bson.M{"lcLabel": 1}},
		{Keys: bson.M{"lcSynonyms": 1}},
		{Keys: bson.M{"refScore": 1}},
		{Keys: bson.M{"taxon": 1}},
		{Keys: bson.M{"definition": "text"}},
		{Keys: bson.M{"instances": 1}},
	}
	if graph == "gene" {
		geneIndices := []mongo.IndexModel{
			{Keys: bson.M{"encodes": 1}},
		}
		dbIndices = append(dbIndices, geneIndices...)
	}

	_, err := entityDB.Indexes().CreateMany(context.TODO(), dbIndices)
	if err != nil {
		panic(err)
	}
	entityNumber := 0
	timestamp := time.Now().Unix()
	for _, entity := range entities {
		entityNumber++
		refScore := refScores[entity.uri]

		lcSynonyms := []string{}
		for _, v := range entity.synonyms {
			lcSynonyms = append(lcSynonyms, strings.ToLower(v))
		}

		var doc bson.M

		if graph == "prot" {
			doc = bson.M{
				"uri":             entity.uri,
				"prefLabel":       entity.prefLabel,
				"lcLabel":         entity.lcLabel,
				"definition":      entity.definition,
				"annotationScore": entity.annotationScore,
				"synonyms":        entity.synonyms,
				"lcSynonyms":      lcSynonyms,
				"instances":       entity.instances,
				"taxon":           taxonPrefix + taxon,
				"refScore":        refScore,
			}
		} else {
			doc = bson.M{
				"uri":             entity.uri,
				"prefLabel":       entity.prefLabel,
				"lcLabel":         entity.lcLabel,
				"definition":      entity.definition,
				"annotationScore": entity.annotationScore,
				"synonyms":        entity.synonyms,
				"lcSynonyms":      lcSynonyms,
				"instances":       entity.instances,
				"taxon":           taxonPrefix + taxon,
				"refScore":        refScore,
				"encodes":         entity.encodes,
			}
		}

		_, err := entityDB.UpdateOne(
			context.TODO(),
			bson.M{"uri": entity.uri},
			bson.M{"$set": doc},
			updateOptions)
		if err != nil {
			panic(err)
		}
		if entityNumber%1000 == 0 {
			nowTime := time.Now().Unix()
			duration := nowTime - timestamp
			timestamp = nowTime
			fmt.Printf("[%s][%s][T%d] Inserted entry %d in %d seconds\n", taxon, graph, index, entityNumber, duration)
			// fmt.Println(taxon, "Thread", index, "Inserted", entityNumber, "into mongoDB graph", graph, "in", duration, "seconds")
		}
	}
}

func cleanRDFString(input string) string {
	components := strings.Split(input, "^^")
	value := strings.Replace(strings.Replace(strings.Replace(components[0], "\"", "", 2), " .", "", 1), "@en", "", 1)
	return value
}

func generateEntityQuery(graph string, constraint string) string {
	query := `SELECT DISTINCT ?uri ?prefLabel ?definition
	WHERE {
		GRAPH <http://rdf.biogateway.eu/graph/%s> {
		?uri skos:prefLabel|rdfs:label ?prefLabel .
		%s
		OPTIONAL { ?uri skos:definition ?definition . }
	}}`
	return fmt.Sprintf(query, graph, constraint)
}
