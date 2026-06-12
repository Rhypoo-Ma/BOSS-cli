package boss

import (
	"strings"
)

// SchoolTier is one of the supported school filtering tiers.
type SchoolTier string

const (
	SchoolTierC9       SchoolTier = "c9"
	SchoolTier985      SchoolTier = "985"
	SchoolTierOverseas SchoolTier = "overseas"
)

// c9Schools is the list of C9 League universities (names and common aliases).
var c9Schools = []string{
	"清华大学", "清华",
	"北京大学", "北大",
	"复旦大学", "复旦",
	"上海交通大学", "上海交大", "上交",
	"南京大学",
	"浙江大学", "浙大",
	"中国科学技术大学", "中科大",
	"哈尔滨工业大学", "哈工大",
	"西安交通大学", "西安交大", "西交",
}

// project985Schools includes all 39 Project 985 universities (names and common aliases).
var project985Schools = append(c9Schools, []string{
	"中国人民大学", "人大",
	"北京航空航天大学", "北航",
	"北京理工大学", "北理",
	"北京师范大学", "北师",
	"中国农业大学",
	"中央民族大学",
	"同济大学",
	"华东师范大学", "华东师范",
	"东南大学",
	"天津大学",
	"南开大学",
	"山东大学",
	"中国海洋大学",
	"武汉大学",
	"华中科技大学", "华中科技", "华科",
	"中南大学",
	"湖南大学",
	"国防科技大学", "国防科大",
	"厦门大学",
	"四川大学",
	"电子科技大学", "电子科大", "成电",
	"重庆大学",
	"西北工业大学", "西北工业", "西工大",
	"西北农林科技大学", "西北农林",
	"兰州大学",
	"中山大学",
	"华南理工大学",
	"吉林大学",
	"大连理工大学",
	"东北大学",
}...)

// overseasSchoolMarkers matches overseas / Hong Kong / Macau / Taiwan schools.
// It combines English institution keywords and well-known non-mainland school names.
var overseasSchoolMarkers = []string{
	// English institution keywords
	"university", "college", "institute of technology", "polytechnic",
	// Hong Kong / Macau / Taiwan
	"香港大学", "港大", "香港中文大学", "港中文", "香港科技大学", "港科大",
	"香港理工大学", "香港城市大学", "香港浸会大学", "岭南大学",
	"澳门大学", "澳门科技大学",
	"台湾大学", "国立台湾大学", "台大",
	// Singapore
	"新加坡国立大学", "国立大学", "南洋理工大学", "南洋理工",
	// Well-known overseas schools (OCR-friendly subset)
	"麻省理工学院", "mit",
	"斯坦福大学", "stanford",
	"哈佛大学", "harvard",
	"剑桥大学", "cambridge",
	"牛津大学", "oxford",
	"哥伦比亚大学", "columbia",
	"密歇根大学", "university of michigan", "michigan",
	"加州大学", "university of california",
	"卡内基梅隆大学", "cmu",
	"多伦多大学", "university of toronto",
	"墨尔本大学", "university of melbourne",
	"悉尼大学", "university of sydney",
	"伦敦大学", "university of london", "帝国理工学院", "imperial college",
	"爱丁堡大学", "university of edinburgh",
	"南洋理工",
}

// SchoolKeywords returns the keyword list for a given school tier.
// If tier is empty or unrecognized, it returns nil.
func SchoolKeywords(tier string) []string {
	switch strings.ToLower(tier) {
	case string(SchoolTierC9):
		return c9Schools
	case string(SchoolTier985):
		return project985Schools
	case string(SchoolTierOverseas):
		return overseasSchoolMarkers
	default:
		return nil
	}
}

// IsOverseasSchool reports whether the school text looks like an overseas institution.
func IsOverseasSchool(text string) bool {
	return extractSchool(text, overseasSchoolMarkers) != ""
}
