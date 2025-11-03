// Copyright 2024 Kuei-chun Chen. All rights reserved.

package mdb

import (
    "fmt"
    "html/template"
    "os"
    "sort"
    "strings"
    "time"

    "go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	htmldir = "./html"
)

// HTMLGenerator generates HTML reports from cluster statistics
type HTMLGenerator struct {
	version string
}

// NewHTMLGenerator returns a new HTMLGenerator
func NewHTMLGenerator(version string) *HTMLGenerator {
	return &HTMLGenerator{version: version}
}

// GenerateClusterHTML generates an HTML report from ClusterStats
func (hg *HTMLGenerator) GenerateClusterHTML(stats *ClusterStats) (string, error) {
	var err error
	os.Mkdir(htmldir, 0755)
	
	basename := stats.HostInfo.System.Hostname
	basename = strings.ReplaceAll(basename, ":", "_")
	ofile := fmt.Sprintf(`%v/%v-stats.html`, htmldir, basename)
	
	i := 1
	for DoesFileExist(ofile) {
		ofile = fmt.Sprintf(`%v/%v.%d-stats.html`, htmldir, basename, i)
		i++
	}

	var w *os.File
	if w, err = os.Create(ofile); err != nil {
		return "", err
	}
	defer w.Close()

	templ, err := hg.GetClusterTemplate()
	if err != nil {
		return "", err
	}

	if err = templ.Execute(w, stats); err != nil {
		return "", err
	}

	fmt.Printf("HTML report written to %v\n", ofile)
	return ofile, nil
}

// GetClusterTemplate returns the HTML template for cluster statistics
func (hg *HTMLGenerator) GetClusterTemplate() (*template.Template, error) {
	return template.New("cluster").Funcs(template.FuncMap{
		"formatBytes":     hg.formatBytes,
		"formatNumber":    hg.formatNumber,
		"formatTime":      hg.formatTime,
		"formatDuration":  hg.formatDuration,
		"getCurrentTime":  func() string { return time.Now().Format("2006-01-02 15:04:05") },
		"getMongoVersion": func() string { return hg.version },
		"div":             func(a, b int64) float64 { return float64(a) / float64(b) },
		"int64":           func(i int) int64 { return int64(i) },
        "add":             func(a, b int) int { return a + b },
        "toInt64":         hg.toInt64,
        // collection helpers
        "fragPct":         hg.fragPct,
        "ratioPct":        hg.ratioPct,
        "indexOps":        hg.indexOps,
        "indexSince":      hg.indexSince,
        "indexOpsOf":      hg.indexOpsOf,
        "indexSinceOf":    hg.indexSinceOf,
        "indexSize":       hg.indexSize,
        "sortCollectionsBySize": hg.sortCollectionsBySize,
        "gtf":             func(a, b float64) bool { return a > b },
	}).Parse(clusterHTMLTemplate)
}

// formatBytes formats bytes into human readable format
func (hg *HTMLGenerator) formatBytes(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatNumber formats numbers with commas
func (hg *HTMLGenerator) formatNumber(n int64) string {
	if n == 0 {
		return "0"
	}
	str := fmt.Sprintf("%d", n)
	if len(str) <= 3 {
		return str
	}
	result := ""
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}

// formatTime formats time
func (hg *HTMLGenerator) formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// formatDuration formats duration in seconds to human readable format
func (hg *HTMLGenerator) formatDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%d seconds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%.1f minutes", float64(seconds)/60)
	}
	if seconds < 86400 {
		return fmt.Sprintf("%.1f hours", float64(seconds)/3600)
	}
	return fmt.Sprintf("%.1f days", float64(seconds)/86400)
}

// toInt64 converts various numeric types to int64 for template usage
func (hg *HTMLGenerator) toInt64(v interface{}) int64 {
    switch x := v.(type) {
    case int64:
        return x
    case int32:
        return int64(x)
    case int:
        return int64(x)
    case float64:
        return int64(x)
    case float32:
        return int64(x)
    default:
        return 0
    }
}

// fragPct returns fragmentation percentage string given data and storage sizes
func (hg *HTMLGenerator) fragPct(dataSize int64, storageSize int64) string {
    if dataSize <= 0 || storageSize <= 0 {
        return "N/A"
    }
    pct := (1.0 - float64(dataSize)/float64(storageSize)) * 100.0
    if pct < 0 {
        pct = 0
    }
    return fmt.Sprintf("%.1f%%", pct)
}

// ratioPct returns index-to-data ratio percentage string
func (hg *HTMLGenerator) ratioPct(indexSize int64, dataSize int64) string {
    if dataSize <= 0 {
        return "N/A"
    }
    pct := (float64(indexSize) / float64(dataSize)) * 100.0
    return fmt.Sprintf("%.1f%%", pct)
}

// indexOps extracts accesses.ops from indexDetails map for a given index name
func (hg *HTMLGenerator) indexOps(details interface{}, name string) int64 {
    m, ok := details.(map[string]interface{})
    if !ok {
        return 0
    }
    if v, ok := m[name]; ok {
        if dm, ok := v.(map[string]interface{}); ok {
            if acc, ok := dm["accesses"].(map[string]interface{}); ok {
                switch x := acc["ops"].(type) {
                case int64:
                    return x
                case int32:
                    return int64(x)
                case int:
                    return int64(x)
                case float64:
                    return int64(x)
                }
            }
        }
    }
    return 0
}

// indexSince extracts accesses.since as string from indexDetails map
func (hg *HTMLGenerator) indexSince(details interface{}, name string) string {
    m, ok := details.(map[string]interface{})
    if !ok {
        return ""
    }
    if v, ok := m[name]; ok {
        if dm, ok := v.(map[string]interface{}); ok {
            if acc, ok := dm["accesses"].(map[string]interface{}); ok {
                return fmt.Sprintf("%v", acc["since"])
            }
        }
    }
    return ""
}

// indexOpsOf returns total ops of an index across shards (or top-level) for a collection
func (hg *HTMLGenerator) indexOpsOf(coll interface{}, name string) int64 {
    c, ok := coll.(Collection)
    if !ok {
        return 0
    }
    var total int64
    // try top-level indexDetails first
    total += hg.indexOps(c.Stats.IndexDetails, name)
    // if sharded, sum per-shard indexDetails
    if c.Stats.Shards != nil {
        for _, v := range c.Stats.Shards {
            if sm, ok := v.(primitive.M); ok {
                total += hg.indexOps(sm["indexDetails"], name)
            }
        }
    }
    return total
}

// indexSinceOf returns the 'since' value from top-level or first shard for a collection
func (hg *HTMLGenerator) indexSinceOf(coll interface{}, name string) string {
    c, ok := coll.(Collection)
    if !ok {
        return ""
    }
    s := hg.indexSince(c.Stats.IndexDetails, name)
    if s != "" {
        return s
    }
    if c.Stats.Shards != nil {
        for _, v := range c.Stats.Shards {
            if sm, ok := v.(primitive.M); ok {
                s = hg.indexSince(sm["indexDetails"], name)
                if s != "" {
                    return s
                }
            }
        }
    }
    return s
}

// indexSize looks up indexSizes[name] and returns it as int64 bytes
func (hg *HTMLGenerator) indexSize(indexSizes interface{}, name string) int64 {
    switch m := indexSizes.(type) {
    case map[string]int64:
        return m[name]
    case map[string]interface{}:
        if v, ok := m[name]; ok {
            return hg.toInt64(v)
        }
    case primitive.M:
        if v, ok := m[name]; ok {
            return hg.toInt64(v)
        }
    }
    return 0
}

// sortCollectionsBySize returns a new slice of collections sorted by stats.size desc
func (hg *HTMLGenerator) sortCollectionsBySize(cols interface{}) []Collection {
    list, ok := cols.([]Collection)
    if !ok {
        return []Collection{}
    }
    out := make([]Collection, len(list))
    copy(out, list)
    sort.Slice(out, func(i, j int) bool { return out[i].Stats.Size > out[j].Stats.Size })
    return out
}

const clusterHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <title>MongoDB Cluster Statistics - {{.HostInfo.System.Hostname}}</title>
  <meta http-equiv="Cache-Control" content="no-cache, no-store, must-revalidate">
  <meta http-equiv="Pragma" content="no-cache">
  <meta http-equiv="Expires" content="0">
  <script src="https://www.gstatic.com/charts/loader.js"></script>
  <style>
    body {
      font-family: Arial, Helvetica, sans-serif;
      margin: 20px;
      background-color: #f5f5f5;
    }
    .container {
      max-width: 1200px;
      margin: 0 auto;
      background-color: white;
      padding: 20px;
      border-radius: 8px;
      box-shadow: 0 2px 4px rgba(0,0,0,0.1);
    }
    h1, h2, h3 {
      color: #333;
      border-bottom: 2px solid #4CAF50;
      padding-bottom: 10px;
    }
    table {
      font-family: Consolas, monaco, monospace;
      border-collapse: collapse;
      width: 100%;
      margin: 10px 0;
    }
    th, td {
      border: 1px solid #ddd;
      padding: 8px;
      text-align: left;
    }
    th {
      background-color: #4CAF50;
      color: white;
      font-weight: bold;
    }
    tr:nth-child(even) {
      background-color: #f2f2f2;
    }
    .summary-card {
      background-color: #e8f5e8;
      border: 1px solid #4CAF50;
      border-radius: 5px;
      padding: 15px;
      margin: 10px 0;
    }
    .metric {
      display: inline-block;
      margin: 5px 15px 5px 0;
      font-weight: bold;
    }
    .chart-container {
      margin: 20px 0;
      padding: 15px;
      border: 1px solid #ddd;
      border-radius: 5px;
    }
    .section {
      margin: 30px 0;
    }
    .timestamp {
      color: #666;
      font-size: 0.9em;
      text-align: right;
    }
  </style>
</head>
<body>
  <div class="container">
    <h1>MongoDB Cluster Statistics</h1>
    <div class="timestamp">Generated: {{getCurrentTime}} | Keyhole Version: {{getMongoVersion}}</div>
    
    <!-- Cluster Summary -->
    <div class="section">
      <h2>Cluster Summary</h2>
      <div class="summary-card">
        <div class="metric">Hostname: {{.HostInfo.System.Hostname}}</div>
        <div class="metric">MongoDB Version: {{.BuildInfo.Version}}</div>
        <div class="metric">Cluster Type: {{.Cluster}}</div>
        <div class="metric">Process: {{.ServerStatus.Process}}</div>
        <div class="metric">OS: {{.HostInfo.OS.Name}}</div>
        <div class="metric">CPU Cores: {{.HostInfo.System.NumCores}}</div>
        <div class="metric">Memory: {{formatBytes (int64 .HostInfo.System.MemSizeMB)}}</div>
        {{if eq .Cluster "sharded"}}
        <div class="metric">Number of Shards: {{len .Shards}}</div>
        {{end}}
      </div>
    </div>

    <!-- Server Status -->
    <div class="section">
      <h2>Server Status</h2>
      <table>
        <tr><th>Metric</th><th>Value</th></tr>
        <tr><td>Host</td><td>{{.ServerStatus.Host}}</td></tr>
        <tr><td>Process</td><td>{{.ServerStatus.Process}}</td></tr>
        <tr><td>Version</td><td>{{.ServerStatus.Version}}</td></tr>
        <tr><td>Connections Current</td><td>{{.ServerStatus.Connections.Current}}</td></tr>
        <tr><td>Connections Available</td><td>{{.ServerStatus.Connections.Available}}</td></tr>
        <tr><td>Connections Total Created</td><td>{{.ServerStatus.Connections.TotalCreated}}</td></tr>
        <tr><td>Memory Resident (MB)</td><td>{{.ServerStatus.Mem.Resident}}</td></tr>
        <tr><td>Memory Virtual (MB)</td><td>{{.ServerStatus.Mem.Virtual}}</td></tr>
        <tr><td>Memory Supported</td><td>{{.ServerStatus.Mem.Supported}}</td></tr>
        <tr><td>Memory Bits</td><td>{{.ServerStatus.Mem.Bits}}</td></tr>
        <tr><td>Local Time</td><td>{{.ServerStatus.LocalTime}}</td></tr>
      </table>
    </div>

    <!-- Operations -->
    <div class="section">
      <h2>Operations</h2>
      <table>
        <tr><th>Operation</th><th>Total</th><th>Rate (ops/sec)</th></tr>
        <tr><td>Insert</td><td>{{formatNumber (int64 .ServerStatus.OpCounters.Insert)}}</td><td>N/A</td></tr>
        <tr><td>Query</td><td>{{formatNumber (int64 .ServerStatus.OpCounters.Query)}}</td><td>N/A</td></tr>
        <tr><td>Update</td><td>{{formatNumber (int64 .ServerStatus.OpCounters.Update)}}</td><td>N/A</td></tr>
        <tr><td>Delete</td><td>{{formatNumber (int64 .ServerStatus.OpCounters.Delete)}}</td><td>N/A</td></tr>
        <tr><td>Get More</td><td>{{formatNumber (int64 .ServerStatus.OpCounters.GetMore)}}</td><td>N/A</td></tr>
        <tr><td>Command</td><td>{{formatNumber (int64 .ServerStatus.OpCounters.Command)}}</td><td>N/A</td></tr>
      </table>
    </div>

    <!-- Databases -->
    {{if .Databases}}
    <div class="section">
      <h2>Databases ({{len .Databases}})</h2>
      <table>
        <tr><th>Database</th><th>Size on Disk</th><th>Empty</th><th>Collections</th></tr>
        {{range .Databases}}
        <tr>
          <td>{{.Name}}</td>
          <td>{{formatBytes .SizeOnDisk}}</td>
          <td>{{.Empty}}</td>
          <td>{{len .Collections}}</td>
        </tr>
        {{end}}
      </table>
    </div>
    {{end}}

    <!-- Collections: Per-Collection Statistics and Index Usage -->
    {{if .Databases}}
    <div class="section">
      <h2>Collections Details</h2>
      {{range .Databases}}
        {{range sortCollectionsBySize .Collections}}
        <h3>{{.NS}}</h3>
        <table>
          <tr><th>Metric</th><th>Value</th></tr>
          <tr><td>Number of Documents</td><td>{{formatNumber .Stats.Count}}</td></tr>
          <tr><td>Average Document Size</td><td>{{formatBytes (toInt64 .Stats.AvgObjSize)}}</td></tr>
          <tr><td>Data Size</td><td>{{formatBytes .Stats.Size}}</td></tr>
          <tr><td>Indexes Size</td><td>{{formatBytes .Stats.TotalIndexSize}}</td></tr>
          <tr><td>Storage Size</td><td>{{formatBytes .Stats.StorageSize}}</td></tr>
          <tr><td>Data File Fragmentation</td><td>{{fragPct .Stats.Size .Stats.StorageSize}}</td></tr>
        </table>

        <!-- Indexes Usage per collection -->
        <h4>Indexes Usage</h4>
        <table>
          <tr><th>#</th><th>key</th><th>size</th><th>host</th><th>ops</th><th>since</th></tr>
          {{ $i := 0 }}
          {{ $coll := . }}
          {{range .Indexes}}
            {{ $i = add $i 1 }}
            <tr>
              <td>{{$i}}</td>
              <td>{{.KeyString}}</td>
              <td>{{formatBytes (indexSize $coll.Stats.IndexSizes .Name)}}</td>
              {{if .Usage}}
                <td>{{(index .Usage 0).Host}}</td>
                <td>{{formatNumber (toInt64 ((index .Usage 0).Accesses.Ops))}}</td>
                <td>{{(index .Usage 0).Accesses.Since}}</td>
              {{else}}
                <td></td>
                <td>0</td>
                <td></td>
              {{end}}
            </tr>
          {{end}}
        </table>
        {{end}}
      {{end}}
    </div>
    {{end}}

    <!-- Shards (if sharded) -->
    {{if eq .Cluster "sharded"}}
    <div class="section">
      <h2>Shards ({{len .Shards}})</h2>
      <table>
        <tr><th>Shard</th><th>Host</th><th>State</th></tr>
        {{range .Shards}}
        <tr>
          <td>{{.ID}}</td>
          <td>{{.Host}}</td>
          <td>{{.State}}</td>
        </tr>
        {{end}}
      </table>
    </div>
    {{end}}

    <!-- Replica Set Status (if replica set) -->
    {{if eq .Cluster "replica"}}
    <div class="section">
      <h2>Replica Set Status</h2>
      <table>
        <tr><th>Member</th><th>State</th><th>Uptime</th><th>Health</th></tr>
        {{range .ReplSetGetStatus.Members}}
        <tr>
          <td>{{.Name}}</td>
          <td>{{.StateStr}}</td>
          <td>{{formatDuration .Uptime}}</td>
          <td>{{.Health}}</td>
        </tr>
        {{end}}
      </table>
    </div>
    {{end}}

    <!-- WiredTiger Cache (if available) -->
    {{if .ServerStatus.WiredTiger}}
    <div class="section">
      <h2>WiredTiger Cache</h2>
      <p>WiredTiger cache information is available but requires specific field mapping.</p>
    </div>
    {{end}}

    <!-- Build Information -->
    <div class="section">
      <h2>Build Information</h2>
      <table>
        <tr><th>Property</th><th>Value</th></tr>
        <tr><td>Version</td><td>{{.BuildInfo.Version}}</td></tr>
        <tr><td>Git Version</td><td>{{.BuildInfo.GitVersion}}</td></tr>
        {{if .BuildInfo.Modules}}
        <tr><td>Modules</td><td>{{range .BuildInfo.Modules}}{{.}} {{end}}</td></tr>
        {{end}}
      </table>
    </div>

    <div class="timestamp">
      <p>Report generated by Keyhole - MongoDB Cluster Analysis Tool</p>
    </div>
  </div>
</body>
</html>`
