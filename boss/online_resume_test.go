package boss

import (
	"testing"
)

func TestSearchResume(t *testing.T) {
	preview := &ResumePreview{
		Name:           "张三",
		Age:            "28岁",
		WorkYears:      "5年",
		EducationLevel: "本科",
		Tags:           []string{"AI产品经理", "机器学习"},
		WorkExperience: []Work{
			{Period: "2020-2024", Company: "字节跳动", Position: "AI算法工程师", Description: "负责推荐系统与NLP模型"},
			{Period: "2018-2020", Company: "百度", Position: "后端开发", Description: "广告系统"},
		},
		EducationHistory: []Edu{
			{Period: "2014-2018", School: "清华大学", Major: "计算机科学", Degree: "本科"},
		},
		Expectation: "AI相关岗位",
		JobTitle:    "AI达人营销",
		RawText:     "姓名：张三\n工作：AI算法工程师\n",
	}

	result := SearchResume(preview, []string{"AI"}, false)
	if !result.Matched {
		t.Fatalf("expected keyword AI to match")
	}
	if result.Count < 4 {
		t.Fatalf("expected at least 4 matches for AI, got %d", result.Count)
	}

	result2 := SearchResume(preview, []string{"区块链"}, false)
	if result2.Matched {
		t.Fatalf("expected keyword 区块链 to not match")
	}

	result3 := SearchResume(preview, []string{""}, false)
	if result3.Matched {
		t.Fatalf("empty keyword should not match")
	}

	result4 := SearchResume(nil, []string{"AI"}, false)
	if result4.Matched {
		t.Fatalf("nil preview should not match")
	}

	// Synonym search
	result5 := SearchResume(preview, []string{"达人", "红人", "KOL"}, false)
	if !result5.Matched {
		t.Fatalf("expected synonym search to match job title AI达人营销")
	}
	foundDaren := false
	for _, m := range result5.Matches {
		if m.Keyword == "达人" {
			foundDaren = true
		}
	}
	if !foundDaren {
		t.Fatalf("expected 达人 to be the matching keyword")
	}

	// Excluding job title should remove 达人 matches from AI达人营销
	result6 := SearchResume(preview, []string{"达人"}, true)
	if result6.Matched {
		t.Fatalf("expected 达人 matches from job title to be excluded")
	}
}
