package main

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"time"
)

//go:embed www/static
var staticFiles embed.FS

//go:embed www/templates
var templateFiles embed.FS

// staticFS is the sub-filesystem rooted at www/static
var staticFS, _ = fs.Sub(staticFiles, "www/static")

// templates holds all parsed page templates
var templates *template.Template

func init() {
	templates = template.Must(template.ParseFS(templateFiles, "www/templates/*.html"))
}

// PageData holds data passed to page templates
type PageData struct {
	Title       string
	Description string
	HeaderSolid bool
	Year        int
	Version     string
	Posts       []BlogPost
}

// BlogPost holds data for a single blog post card
type BlogPost struct {
	Title    string
	Excerpt  string
	Category string
	Date     string
	ReadTime string
	Icon     string
}

// handleIndex renders the homepage
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Year:    time.Now().Year(),
		Version: Version,
	}
	if err := templates.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleScan renders the scan page
func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:       "Scan",
		Description: "Run a custom SEO and AEO audit with selected analysis skills.",
		HeaderSolid: true,
		Year:        time.Now().Year(),
		Version:     Version,
	}
	if err := templates.ExecuteTemplate(w, "scan.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleReports renders the reports browser page
func (s *Server) handleReports(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:       "Reports",
		Description: "Browse published reports.",
		HeaderSolid: true,
		Year:        time.Now().Year(),
		Version:     Version,
	}
	if err := templates.ExecuteTemplate(w, "reports.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleBlog renders the blog page
func (s *Server) handleBlog(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:       "Blog",
		Description: "SEO and AEO insights, tips, and strategies from the Optimus team.",
		HeaderSolid: true,
		Year:        time.Now().Year(),
		Version:     Version,
		Posts: []BlogPost{
			{
				Title:    "What Is AEO and Why Your Business Needs It in 2026",
				Excerpt:  "AI Engine Optimization is the next frontier of search. Learn how to position your business to be recommended by AI assistants like ChatGPT, Gemini, and Claude.",
				Category: "AEO",
				Date:     "March 15, 2026",
				ReadTime: "8 min read",
				Icon:     "fa-robot",
			},
			{
				Title:    "Technical SEO Checklist: 25 Items You Can't Ignore",
				Excerpt:  "From Core Web Vitals to structured data, this comprehensive checklist covers every technical SEO factor that impacts your rankings and user experience.",
				Category: "Technical SEO",
				Date:     "March 8, 2026",
				ReadTime: "12 min read",
				Icon:     "fa-cogs",
			},
			{
				Title:    "How We Increased Organic Traffic by 400% in 6 Months",
				Excerpt:  "A detailed case study of how we helped a B2B SaaS company transform their organic search presence and drive qualified leads through strategic SEO.",
				Category: "Case Study",
				Date:     "February 28, 2026",
				ReadTime: "10 min read",
				Icon:     "fa-chart-line",
			},
			{
				Title:    "The Complete Guide to Building Topical Authority",
				Excerpt:  "Topical authority is how Google determines expertise. Learn our proven framework for building content clusters that establish your site as the go-to resource.",
				Category: "Content Strategy",
				Date:     "February 20, 2026",
				ReadTime: "15 min read",
				Icon:     "fa-sitemap",
			},
			{
				Title:    "Local SEO in 2026: What's Changed and What Works Now",
				Excerpt:  "Google's local algorithm has evolved significantly. Discover the strategies that are driving results for local businesses right now.",
				Category: "Local SEO",
				Date:     "February 12, 2026",
				ReadTime: "7 min read",
				Icon:     "fa-map-marker-alt",
			},
			{
				Title:    "Link Building Strategies That Actually Work in 2026",
				Excerpt:  "Forget outdated tactics. These ethical, scalable link building strategies will help you earn high-authority backlinks that move the needle.",
				Category: "Link Building",
				Date:     "February 5, 2026",
				ReadTime: "9 min read",
				Icon:     "fa-link",
			},
		},
	}
	if err := templates.ExecuteTemplate(w, "blog.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
