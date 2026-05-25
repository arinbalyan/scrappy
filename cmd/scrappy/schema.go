package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/arinbalyan/scrappy/internal/model"
)

// jobPostSchema builds a JSON Schema-compatible description of the JobPost type.
func jobPostSchema() map[string]interface{} {
	m := map[string]interface{}{
		"$schema":     "https://json-schema.org/draft-07/schema#",
		"title":       "JobPost",
		"description": "A single scraped job posting from any supported board",
		"type":        "object",
		"properties":  buildProperties(reflect.TypeOf(model.JobPost{})),
	}
	m["required"] = []string{"title", "job_url", "site"}
	return m
}

func buildProperties(t reflect.Type) map[string]interface{} {
	props := map[string]interface{}{}
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("json")
		name, opts, _ := strings.Cut(tag, ",")
		if name == "" || name == "-" {
			continue
		}
		omitempty := strings.Contains(opts, "omitempty")
		prop := map[string]interface{}{}
		prop["description"] = fieldDescription(name, f.Name)
		switch f.Type.Kind() {
		case reflect.String:
			prop["type"] = "string"
		case reflect.Bool:
			prop["type"] = "boolean"
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			prop["type"] = "integer"
		case reflect.Float32, reflect.Float64:
			prop["type"] = "number"
		case reflect.Slice:
			prop["type"] = "array"
			prop["items"] = elemType(f.Type.Elem())
		case reflect.Struct:
			prop["type"] = "object"
			prop["properties"] = buildProperties(f.Type)
		case reflect.Ptr:
			prop["nullable"] = true
			if f.Type.Elem().Kind() == reflect.Struct {
				prop["type"] = "object"
				prop["properties"] = buildProperties(f.Type.Elem())
			} else if f.Type.Elem().Kind() == reflect.Float64 {
				prop["type"] = "number"
			} else {
				prop["type"] = "string"
			}
		case reflect.Map:
			prop["type"] = "object"
			prop["additionalProperties"] = map[string]string{"type": "string"}
		default:
			prop["type"] = "string"
		}
		if omitempty {
			prop["optional"] = true
		}
		if ex := exampleValue(name); ex != nil {
			prop["examples"] = []interface{}{ex}
		}
		props[name] = prop
	}
	return props
}

func fieldDescription(jsonName, goName string) string {
	desc := map[string]string{
		"id":                   "Unique identifier for the posting (site-specific)",
		"title":                "Job title",
		"company_name":         "Name of the hiring company",
		"company_url":          "Company's website URL",
		"job_url":              "Direct URL to the job posting",
		"job_url_direct":       "Direct application URL (bypasses board)",
		"location":             "Geographic location of the job (city, state, country)",
		"is_remote":            "Whether the job is flagged as remote",
		"description":          "Full job description text (HTML stripped)",
		"job_type":             "Employment type: fulltime, parttime, contract, internship, temporary",
		"date_posted":          "When the job was posted (RFC3339)",
		"site":                 "Source job board name",
		"fetched_at":           "When scrappy scraped the posting (RFC3339)",
		"emails":               "List of extracted contact email addresses with verification status",
		"compensation":         "Salary information: interval, min_amount, max_amount, currency",
		"seniority":            "Seniority level: entry, mid, senior, lead",
		"department":           "Department: eng, data, product, etc.",
		"domain":               "Company domain extracted from email or URL",
		"industry":             "Company industry classification",
		"company_logo_url":     "URL to the company's logo image",
		"apply_method":         "How to apply: easy_apply, email, external_url",
		"job_level":            "LinkedIn-specific job level classification",
		"company_industry":     "Industry classification from LinkedIn or Indeed",
		"company_addresses":    "Company physical addresses (Indeed)",
		"company_num_employees": "Number of employees (Indeed)",
		"company_revenue":      "Company revenue range (Indeed)",
		"company_description":  "Company description text",
		"company_logo":         "Company logo URL or base64 (site-specific)",
		"skills":               "List of skills mentioned in the posting",
		"experience_range":     "Required experience range (e.g. '3-5 years')",
		"company_rating":       "Company rating/score out of 5",
		"company_reviews_count": "Number of company reviews",
		"vacancy_count":        "Number of open vacancies for this role",
		"work_from_home_type":  "Work-from-home classification (hybrid, fully remote, etc.)",
		"quality_score":        "Computed quality score (0-100)",
	}
	if d, ok := desc[jsonName]; ok {
		return d
	}
	return fmt.Sprintf("Field: %s", goName)
}

func elemType(t reflect.Type) interface{} {
	switch t.Kind() {
	case reflect.String:
		return map[string]string{"type": "string"}
	case reflect.Struct:
		return map[string]interface{}{
			"type":       "object",
			"properties": buildProperties(t),
		}
	default:
		return map[string]string{"type": "string"}
	}
}

func exampleValue(name string) interface{} {
	examples := map[string]interface{}{
		"title":        "Senior Software Engineer",
		"company_name": "Acme Corp",
		"job_type":     "fulltime",
		"site":         "linkedin",
		"is_remote":    true,
		"seniority":    "senior",
		"department":   "engineering",
		"apply_method": "easy_apply",
		"industry":     "Information Technology",
		"quality_score": 78,
	}
	if v, ok := examples[name]; ok {
		return v
	}
	return nil
}

// printSchema outputs the JobPost JSON Schema to stdout.
func printSchema() {
	schema := jobPostSchema()
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(schema); err != nil {
		fmt.Fprintf(os.Stderr, "error printing schema: %v\n", err)
		os.Exit(1)
	}
}
