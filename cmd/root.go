package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Rhypoo-Ma/BOSS-cli/boss"
	"github.com/Rhypoo-Ma/BOSS-cli/browser"
	"github.com/Rhypoo-Ma/BOSS-cli/output"
	"github.com/spf13/cobra"
)

var debug bool

var rootCmd = &cobra.Command{
	Use:   "BOSS-cli",
	Short: "BOSS Zhipin automation CLI",
}

func Execute() error {
	return rootCmd.Execute()
}

func newClient() *browser.Client {
	client := browser.NewClient("BOSS-cli")
	client.Debug = debug
	return client
}

// parseKeywords splits a comma-separated keyword string and trims empty entries.
func parseKeywords(s string) []string {
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func handleError(code, message string, client *browser.Client) {
	if debug && client != nil {
		if snap, err := client.Snapshot(); err == nil {
			fmt.Fprintf(os.Stderr, "\n--- debug snapshot ---\n%s\n--- end snapshot ---\n", snap)
		}
	}
	output.Error(code, message)
	os.Exit(1)
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Print diagnostic snapshots on errors")

	// login-status
	loginStatusCmd := &cobra.Command{
		Use:   "login-status",
		Short: "Check BOSS login status",
		Run: func(cmd *cobra.Command, args []string) {
			client := newClient()
			status, err := boss.CheckLogin(client)
			if err != nil {
				handleError("login_check_failed", err.Error(), client)
			}
			if !status.LoggedIn {
				handleError("not_logged_in", "Not logged in to BOSS Zhipin. Please open Chrome, navigate to https://www.zhipin.com, and log in manually. Then run this command again.", client)
			}
			output.Success(status)
		},
	}
	rootCmd.AddCommand(loginStatusCmd)

	// list-jobs
	listJobsCmd := &cobra.Command{
		Use:   "list-jobs",
		Short: "List jobs in chat page",
		Run: func(cmd *cobra.Command, args []string) {
			client := newClient()
			_, err := boss.CheckLogin(client)
			if err != nil {
				handleError("not_logged_in", "Please run login-status first and ensure you are logged in.", client)
			}
			jobs, err := boss.ListJobs(client)
			if err != nil {
				handleError("list_jobs_failed", err.Error(), client)
			}
			output.Success(jobs)
		},
	}
	rootCmd.AddCommand(listJobsCmd)

	// switch-job
	switchJobCmd := &cobra.Command{
		Use:   "switch-job [job-name]",
		Short: "Switch to a specific job filter with optional status and unread filters",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filterStatus, _ := cmd.Flags().GetString("filter")
			unreadOnly, _ := cmd.Flags().GetBool("unread")
			client := newClient()
			_, err := boss.CheckLogin(client)
			if err != nil {
				handleError("not_logged_in", "Please run login-status first and ensure you are logged in.", client)
			}
			result, err := boss.SwitchJobWithFilters(client, args[0], filterStatus, unreadOnly)
			if err != nil {
				handleError("switch_job_failed", err.Error(), client)
			}
			output.Success(result)
		},
	}
	switchJobCmd.Flags().String("filter", "全部", "Communication status filter: 全部, 新招呼, 沟通中, 已约面, 已获取简历, 已交换电话, 已交换微信, 收藏, 更多")
	switchJobCmd.Flags().Bool("unread", false, "Only show unread messages")
	rootCmd.AddCommand(switchJobCmd)

	// list-candidates
	listCandidatesCmd := &cobra.Command{
		Use:   "list-candidates",
		Short: "List candidates in current job filter",
		Run: func(cmd *cobra.Command, args []string) {
			status, _ := cmd.Flags().GetString("status")
			all, _ := cmd.Flags().GetBool("all")
			max, _ := cmd.Flags().GetInt("max")
			client := newClient()
			_, err := boss.CheckLogin(client)
			if err != nil {
				handleError("not_logged_in", "Please run login-status first and ensure you are logged in.", client)
			}
			candidates, err := boss.ListCandidates(client, strings.TrimSpace(status), all, max)
			if err != nil {
				handleError("list_candidates_failed", err.Error(), client)
			}
			output.Success(candidates)
		},
	}
	listCandidatesCmd.Flags().String("status", "", "Filter by status: 新招呼, 沟通中, 已约面, 已获取简历, 已交换电话, 已交换微信, 收藏, 更多")
	listCandidatesCmd.Flags().Bool("all", false, "Scroll through the virtual list and load all visible candidates")
	listCandidatesCmd.Flags().Int("max", 0, "Maximum number of candidates to load (0 = unlimited, requires --all)")
	rootCmd.AddCommand(listCandidatesCmd)

	// send-message
	sendMessageCmd := &cobra.Command{
		Use:   "send-message [candidate-name] [message]",
		Short: "Send a message to a candidate",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			client := newClient()
			_, err := boss.CheckLogin(client)
			if err != nil {
				handleError("not_logged_in", "Please run login-status first and ensure you are logged in.", client)
			}
			message := strings.Join(args[1:], " ")
			if err := boss.SendMessage(client, args[0], message); err != nil {
				handleError("send_message_failed", err.Error(), client)
			}
			output.Success(map[string]string{"candidate": args[0], "sent": "true"})
		},
	}
	rootCmd.AddCommand(sendMessageCmd)

	// download-resume
	downloadResumeCmd := &cobra.Command{
		Use:   "download-resume [candidate-name]",
		Short: "Click to download a candidate's resume",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dir, _ := cmd.Flags().GetString("dir")
			client := newClient()
			_, err := boss.CheckLogin(client)
			if err != nil {
				handleError("not_logged_in", "Please run login-status first and ensure you are logged in.", client)
			}
			if err := boss.DownloadResume(client, args[0], dir); err != nil {
				handleError("download_resume_failed", err.Error(), client)
			}
			output.Success(map[string]string{"candidate": args[0], "downloaded": "true"})
		},
	}
	downloadResumeCmd.Flags().String("dir", "", "Download directory (optional, browser default used)")
	rootCmd.AddCommand(downloadResumeCmd)

	// view-resume
	viewResumeCmd := &cobra.Command{
		Use:   "view-resume [candidate-name]",
		Short: "Open a candidate's online resume and extract preview info",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			keepOpen, _ := cmd.Flags().GetBool("keep-open")
			client := newClient()
			_, err := boss.CheckLogin(client)
			if err != nil {
				handleError("not_logged_in", "Please run login-status first and ensure you are logged in.", client)
			}
			_, err = boss.OpenOnlineResume(client, args[0])
			if err != nil {
				handleError("open_resume_failed", err.Error(), client)
			}
			preview, err := boss.ExtractResumePreview(client)
			if err != nil {
				handleError("extract_resume_failed", err.Error(), client)
			}
			if !keepOpen {
				_ = boss.CloseOnlineResume(client)
			}
			keyword, _ := cmd.Flags().GetString("keyword")
			useOCR, _ := cmd.Flags().GetBool("ocr")
			excludeJobTitle, _ := cmd.Flags().GetBool("exclude-job-title")
			if keyword == "" {
				output.Success(preview)
				return
			}
			keywords := parseKeywords(keyword)
			if useOCR {
				ocrResult, err := boss.SearchResumeWithOCR(client, args[0], keywords, excludeJobTitle)
				if err != nil {
					handleError("ocr_search_failed", err.Error(), client)
				}
				output.Success(ocrResult)
			} else {
				output.Success(boss.SearchResume(preview, keywords, excludeJobTitle))
			}
		},
	}
	viewResumeCmd.Flags().Bool("keep-open", false, "Keep the resume dialog open after extraction")
	viewResumeCmd.Flags().String("keyword", "", "Comma-separated keywords to search in the resume")
	viewResumeCmd.Flags().Bool("ocr", false, "Use OCR on the online resume screenshot for keyword search (macOS only)")
	viewResumeCmd.Flags().Bool("exclude-job-title", false, "Ignore matches that only come from the job title 'AI达人营销'")
	rootCmd.AddCommand(viewResumeCmd)

	// scan-resumes
	scanResumesCmd := &cobra.Command{
		Use:   "scan-resumes [job-name]",
		Short: "Scan candidates in a job/filter and send message if keyword matches",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filter, _ := cmd.Flags().GetString("filter")
			unread, _ := cmd.Flags().GetBool("unread")
			keyword, _ := cmd.Flags().GetString("keyword")
			message, _ := cmd.Flags().GetString("message")
			useOCR, _ := cmd.Flags().GetBool("ocr")
			excludeJobTitle, _ := cmd.Flags().GetBool("exclude-job-title")
			max, _ := cmd.Flags().GetInt("max")
			minGrade, _ := cmd.Flags().GetInt("min-grade")
			names, _ := cmd.Flags().GetString("names")
			schoolTier, _ := cmd.Flags().GetString("school-tier")
			schools, _ := cmd.Flags().GetString("schools")
			if keyword == "" {
				handleError("missing_keyword", "Please provide --keyword", nil)
			}
			if message == "" {
				handleError("missing_message", "Please provide --message", nil)
			}
			client := newClient()
			if _, err := boss.CheckLogin(client); err != nil {
				handleError("not_logged_in", "Please run login-status first and ensure you are logged in.", client)
			}
			results, err := boss.ScanResumes(client, args[0], filter, unread, parseKeywords(keyword), message, useOCR, excludeJobTitle, max, minGrade, parseKeywords(names), schoolTier, parseKeywords(schools))
			if err != nil {
				handleError("scan_failed", err.Error(), client)
			}
			output.Success(results)
		},
	}
	scanResumesCmd.Flags().String("filter", "", "Communication status filter (e.g. 新招呼)")
	scanResumesCmd.Flags().Bool("unread", false, "Only scan unread candidates")
	scanResumesCmd.Flags().String("keyword", "", "Comma-separated keywords to search in resumes")
	scanResumesCmd.Flags().String("message", "", "Message to send when a keyword matches")
	scanResumesCmd.Flags().Bool("ocr", false, "Use OCR on the online resume screenshot for keyword search (macOS only)")
	scanResumesCmd.Flags().Bool("exclude-job-title", false, "Ignore matches that only come from the job title")
	scanResumesCmd.Flags().Int("max", 50, "Maximum candidates to scan (0 = no limit)")
	scanResumesCmd.Flags().Int("min-grade", 0, "Minimum graduation year (e.g. 2027). 0 means no filter.")
	scanResumesCmd.Flags().String("names", "", "Comma-separated candidate names to scan (default: all)")
	scanResumesCmd.Flags().String("school-tier", "", "School tier preset: c9, 985, or overseas (can combine with --schools)")
	scanResumesCmd.Flags().String("schools", "", "Extra comma-separated school keywords to filter candidates (e.g. 清华,北大,浙大)")
	rootCmd.AddCommand(scanResumesCmd)

	// close-resume
	closeResumeCmd := &cobra.Command{
		Use:   "close-resume",
		Short: "Close the online resume dialog",
		Run: func(cmd *cobra.Command, args []string) {
			client := newClient()
			if err := boss.CloseOnlineResume(client); err != nil {
				handleError("close_resume_failed", err.Error(), client)
			}
			output.Success(map[string]string{"closed": "true"})
		},
	}
	rootCmd.AddCommand(closeResumeCmd)

	// scroll-resume
	scrollResumeCmd := &cobra.Command{
		Use:   "scroll-resume [pixels]",
		Short: "Scroll the online resume dialog down by the given pixels",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			pixels := 0
			fmt.Sscanf(args[0], "%d", &pixels)
			if pixels <= 0 {
				pixels = 500
			}
			client := newClient()
			if err := boss.ScrollOnlineResume(client, pixels); err != nil {
				handleError("scroll_resume_failed", err.Error(), client)
			}
			output.Success(map[string]int{"scrolled": pixels})
		},
	}
	rootCmd.AddCommand(scrollResumeCmd)
}
