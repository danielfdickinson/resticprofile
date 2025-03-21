package config

import (
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/creativeprojects/resticprofile/restic"
	"github.com/creativeprojects/resticprofile/util/collect"
	"github.com/creativeprojects/resticprofile/util/templates"
)

// TemplateData contain the variables fed to a config template
type TemplateData struct {
	templates.DefaultData
	Profile   ProfileTemplateData
	Schedule  ScheduleTemplateData
	ConfigDir string
}

// ProfileTemplateData contains profile data
type ProfileTemplateData struct {
	Name string
}

// ScheduleTemplateData contains schedule data
type ScheduleTemplateData struct {
	Name string
}

// newTemplateData populates a TemplateData struct ready to use
func newTemplateData(configFile, profileName, scheduleName string) TemplateData {
	configDir := filepath.Dir(configFile)
	if !filepath.IsAbs(configDir) {
		currentDir, _ := os.Getwd()
		configDir = filepath.Join(currentDir, configDir)
	}
	configDir = filepath.ToSlash(configDir)

	return TemplateData{
		DefaultData: templates.NewDefaultData(nil),
		Profile: ProfileTemplateData{
			Name: profileName,
		},
		Schedule: ScheduleTemplateData{
			Name: scheduleName,
		},
		ConfigDir: configDir,
	}
}

// TemplateInfoData is used as data for go templates that render config references
type TemplateInfoData struct {
	templates.DefaultData
	Global, Group       PropertySet
	Profile             ProfileInfo
	KnownResticVersions []string
}

// ProfileSections is a helper method for templates to list SectionInfo in ProfileInfo
func (t *TemplateInfoData) ProfileSections() []SectionInfo {
	return collect.From(t.Profile.Sections(), t.Profile.SectionInfo)
}

func (t *TemplateInfoData) collectPropertiesByType(set PropertySet, byType map[string]*namedPropertySet) {
	properties := collect.From(set.Properties(), set.PropertyInfo)
	if other := set.OtherPropertyInfo(); other != nil {
		properties = append(properties, other)
	}
	for _, property := range properties {
		if nested := property.PropertySet(); nested != nil && len(nested.TypeName()) > 0 {
			if nps, ok := nested.(*namedPropertySet); ok {
				byType[nested.TypeName()] = nps
			}
			t.collectPropertiesByType(nested, byType)
		}
	}
}

// NestedSections lists SectionInfo of all nested sections that may be used inside the configuration
func (t *TemplateInfoData) NestedSections() []SectionInfo {
	sectionByType := make(map[string]*namedPropertySet)

	t.collectPropertiesByType(t.Global, sectionByType)
	t.collectPropertiesByType(t.Group, sectionByType)
	t.collectPropertiesByType(t.Profile, sectionByType)
	for _, section := range t.ProfileSections() {
		t.collectPropertiesByType(section, sectionByType)
	}

	typeNames := slices.Sorted(maps.Keys(sectionByType))

	return collect.From(typeNames, func(name string) SectionInfo {
		section := sectionByType[name]
		return &sectionInfo{
			namedPropertySet: namedPropertySet{
				name:        name,
				description: section.Description(),
				propertySet: section.propertySet,
			},
		}
	})
}

// GetFuncs returns a map of helpers to be used as methods when rendering templates
func (t *TemplateInfoData) GetFuncs() map[string]any {
	return map[string]any{
		"properties": func(set PropertySet) []PropertyInfo { return collect.From(set.Properties(), set.PropertyInfo) },
		"own":        func(p []PropertyInfo) []PropertyInfo { return collect.All(p, collect.Not(PropertyInfo.IsOption)) },
		"restic":     func(p []PropertyInfo) []PropertyInfo { return collect.All(p, PropertyInfo.IsOption) },
	}
}

// NewTemplateInfoData returns template data to render references for the specified resticVersion
func NewTemplateInfoData(resticVersion string) *TemplateInfoData {
	return &TemplateInfoData{
		DefaultData:         templates.NewDefaultData(nil),
		Global:              NewGlobalInfo(),
		Group:               NewGroupInfo(),
		Profile:             NewProfileInfoForRestic(resticVersion, false),
		KnownResticVersions: restic.KnownVersions(),
	}
}
