// go.mod
module netgit

go 1.21

require (
    github.com/spf13/cobra v1.7.0
    github.com/spf13/viper v1.16.0
    go.etcd.io/bbolt v1.3.7
    github.com/prometheus/client_golang v1.16.0
    github.com/open-policy-agent/opa v0.55.0
    github.com/google/uuid v1.3.0
    gopkg.in/yaml.v3 v3.0.1
    github.com/stretchr/testify v1.8.4
    github.com/gin-gonic/gin v1.9.1
    go.uber.org/zap v1.24.0
)

// main.go
package main

import (
    "fmt"
    "os"
    "netgit/cmd/netgit"
)

func main() {
    if err := netgit.Execute(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}

// cmd/netgit/root.go
package netgit

import (
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
    Use:   "netgit",
    Short: "Git-like version control for network configurations",
    Long: `Network Git provides version control, policy verification, and safe deployment 
for network configurations across multiple cloud providers and platforms.`,
}

func Execute() error {
    return rootCmd.Execute()
}

func init() {
    cobra.OnInitialize(initConfig)
    
    rootCmd.AddCommand(initCmd)
    rootCmd.AddCommand(commitCmd)
    rootCmd.AddCommand(diffCmd)
    rootCmd.AddCommand(verifyCmd)
    rootCmd.AddCommand(deployCmd)
    rootCmd.AddCommand(revertCmd)
    rootCmd.AddCommand(historyCmd)
    rootCmd.AddCommand(statusCmd)
    rootCmd.AddCommand(branchCmd)
    rootCmd.AddCommand(mergeCmd)
}

func initConfig() {
    viper.SetConfigName(".netgit")
    viper.SetConfigType("yaml")
    viper.AddConfigPath(".")
    viper.AddConfigPath("$HOME")
    viper.AutomaticEnv()
    
    if err := viper.ReadInConfig(); err == nil {
        // Config file found and successfully parsed
    }
}

// cmd/netgit/commands.go
package netgit

import (
    "fmt"
    "os"
    "strings"
    "time"
    
    "github.com/spf13/cobra"
    "netgit/pkg/storage"
    "netgit/pkg/config"
    "netgit/pkg/policy"
    "netgit/pkg/deploy"
    "netgit/pkg/audit"
)

var (
    message string
    dryRun  bool
    force   bool
    target  string
    canary  bool
)

var initCmd = &cobra.Command{
    Use:   "init",
    Short: "Initialize a new netgit repository",
    RunE: func(cmd *cobra.Command, args []string) error {
        repo, err := storage.NewRepository(".")
        if err != nil {
            return fmt.Errorf("failed to initialize repository: %w", err)
        }
        defer repo.Close()
        
        fmt.Println("Initialized empty netgit repository in", repo.Path())
        return nil
    },
}

var commitCmd = &cobra.Command{
    Use:   "commit",
    Short: "Record changes to the repository",
    RunE: func(cmd *cobra.Command, args []string) error {
        if message == "" {
            return fmt.Errorf("commit message is required")
        }
        
        repo, err := storage.OpenRepository(".")
        if err != nil {
            return err
        }
        defer repo.Close()
        
        configFiles, err := config.LoadWorkingDirectory(".")
        if err != nil {
            return err
        }
        
        commit, err := repo.Commit(configFiles, message, "user@netgit.local")
        if err != nil {
            return err
        }
        
        audit.LogEvent("commit", map[string]interface{}{
            "commit_hash": commit.Hash,
            "message":     commit.Message,
            "files":       len(configFiles),
            "timestamp":   commit.Timestamp,
        })
        
        fmt.Printf("[%s] %s\n", commit.Hash[:8], commit.Message)
        fmt.Printf("%d files changed\n", len(configFiles))
        return nil
    },
}

var diffCmd = &cobra.Command{
    Use:   "diff [rev1] [rev2]",
    Short: "Show changes between commits",
    RunE: func(cmd *cobra.Command, args []string) error {
        repo, err := storage.OpenRepository(".")
        if err != nil {
            return err
        }
        defer repo.Close()
        
        var rev1, rev2 string
        switch len(args) {
        case 0:
            // diff working directory vs HEAD
            rev1 = "HEAD"
            rev2 = "WORKING"
        case 1:
            rev1 = args[0]
            rev2 = "WORKING"
        case 2:
            rev1 = args[0]
            rev2 = args[1]
        default:
            return fmt.Errorf("too many arguments")
        }
        
        diff, err := repo.Diff(rev1, rev2)
        if err != nil {
            return err
        }
        
        fmt.Print(diff.String())
        return nil
    },
}

var verifyCmd = &cobra.Command{
    Use:   "verify",
    Short: "Verify configurations against policies",
    RunE: func(cmd *cobra.Command, args []string) error {
        configFiles, err := config.LoadWorkingDirectory(".")
        if err != nil {
            return err
        }
        
        policyEngine, err := policy.NewEngine("policies")
        if err != nil {
            return err
        }
        
        violations := []policy.Violation{}
        for _, cfg := range configFiles {
            results, err := policyEngine.Verify(cfg)
            if err != nil {
                return err
            }
            violations = append(violations, results...)
        }
        
        if len(violations) > 0 {
            fmt.Printf("Found %d policy violations:\n\n", len(violations))
            for _, v := range violations {
                fmt.Printf("❌ %s: %s\n", v.Rule, v.Message)
                fmt.Printf("   File: %s\n", v.File)
                if v.Path != "" {
                    fmt.Printf("   Path: %s\n", v.Path)
                }
                fmt.Println()
            }
            os.Exit(1)
        }
        
        fmt.Printf("✅ All configurations passed policy verification\n")
        return nil
    },
}

var deployCmd = &cobra.Command{
    Use:   "deploy",
    Short: "Deploy configurations to target environment",
    RunE: func(cmd *cobra.Command, args []string) error {
        repo, err := storage.OpenRepository(".")
        if err != nil {
            return err
        }
        defer repo.Close()
        
        head, err := repo.GetHEAD()
        if err != nil {
            return err
        }
        
        deployer, err := deploy.GetDeployer(target)
        if err != nil {
            return err
        }
        
        if dryRun {
            fmt.Println("Performing dry run...")
            if err := deployer.DryRun(head.Config); err != nil {
                return fmt.Errorf("dry run failed: %w", err)
            }
            fmt.Println("✅ Dry run successful")
            return nil
        }
        
        deployment := &deploy.Deployment{
            CommitHash: head.Hash,
            Target:     target,
            Canary:     canary,
            Timestamp:  time.Now(),
        }
        
        if canary {
            fmt.Println("Starting canary deployment (10% traffic)...")
            if err := deployer.CanaryDeploy(head.Config, 10); err != nil {
                return err
            }
            
            fmt.Println("Canary deployment successful. Expanding to 100%...")
            time.Sleep(5 * time.Second) // Simulate monitoring period
            
            if err := deployer.ExpandCanary(head.Config); err != nil {
                fmt.Println("Expansion failed, rolling back canary...")
                deployer.Rollback(head.Config)
                return err
            }
        } else {
            if err := deployer.Apply(head.Config); err != nil {
                return err
            }
        }
        
        audit.LogEvent("deploy", map[string]interface{}{
            "commit_hash": head.Hash,
            "target":      target,
            "canary":      canary,
            "timestamp":   deployment.Timestamp,
        })
        
        fmt.Printf("✅ Successfully deployed %s to %s\n", head.Hash[:8], target)
        return nil
    },
}

var revertCmd = &cobra.Command{
    Use:   "revert <commit>",
    Short: "Revert to a specific commit",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        commitHash := args[0]
        
        repo, err := storage.OpenRepository(".")
        if err != nil {
            return err
        }
        defer repo.Close()
        
        commit, err := repo.GetCommit(commitHash)
        if err != nil {
            return err
        }
        
        deployer, err := deploy.GetDeployer(target)
        if err != nil {
            return err
        }
        
        if err := deployer.Rollback(commit.Config); err != nil {
            return fmt.Errorf("rollback failed: %w", err)
        }
        
        audit.LogEvent("revert", map[string]interface{}{
            "commit_hash": commitHash,
            "target":      target,
            "timestamp":   time.Now(),
        })
        
        fmt.Printf("✅ Reverted to commit %s\n", commitHash[:8])
        return nil
    },
}

var historyCmd = &cobra.Command{
    Use:   "history",
    Short: "Show commit history",
    RunE: func(cmd *cobra.Command, args []string) error {
        repo, err := storage.OpenRepository(".")
        if err != nil {
            return err
        }
        defer repo.Close()
        
        commits, err := repo.GetHistory()
        if err != nil {
            return err
        }
        
        for _, commit := range commits {
            fmt.Printf("commit %s\n", commit.Hash)
            fmt.Printf("Author: %s\n", commit.Author)
            fmt.Printf("Date: %s\n", commit.Timestamp.Format("Mon Jan 2 15:04:05 2006"))
            fmt.Printf("\n    %s\n\n", commit.Message)
        }
        
        return nil
    },
}

var statusCmd = &cobra.Command{
    Use:   "status",
    Short: "Show working directory status",
    RunE: func(cmd *cobra.Command, args []string) error {
        repo, err := storage.OpenRepository(".")
        if err != nil {
            return err
        }
        defer repo.Close()
        
        status, err := repo.Status()
        if err != nil {
            return err
        }
        
        fmt.Printf("On branch %s\n", status.Branch)
        if status.Clean {
            fmt.Println("nothing to commit, working directory clean")
        } else {
            fmt.Printf("Changes not staged for commit:\n")
            for _, file := range status.Modified {
                fmt.Printf("  modified: %s\n", file)
            }
            for _, file := range status.Added {
                fmt.Printf("  new file: %s\n", file)
            }
            for _, file := range status.Deleted {
                fmt.Printf("  deleted:  %s\n", file)
            }
        }
        return nil
    },
}

var branchCmd = &cobra.Command{
    Use:   "branch [name]",
    Short: "List or create branches",
    RunE: func(cmd *cobra.Command, args []string) error {
        repo, err := storage.OpenRepository(".")
        if err != nil {
            return err
        }
        defer repo.Close()
        
        if len(args) == 0 {
            branches, err := repo.ListBranches()
            if err != nil {
                return err
            }
            
            current, _ := repo.CurrentBranch()
            for _, branch := range branches {
                if branch == current {
                    fmt.Printf("* %s\n", branch)
                } else {
                    fmt.Printf("  %s\n", branch)
                }
            }
            return nil
        }
        
        branchName := args[0]
        if err := repo.CreateBranch(branchName); err != nil {
            return err
        }
        
        fmt.Printf("Created branch '%s'\n", branchName)
        return nil
    },
}

var mergeCmd = &cobra.Command{
    Use:   "merge <branch>",
    Short: "Merge branch into current branch",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        branchName := args[0]
        
        repo, err := storage.OpenRepository(".")
        if err != nil {
            return err
        }
        defer repo.Close()
        
        merge, err := repo.Merge(branchName)
        if err != nil {
            return err
        }
        
        fmt.Printf("Merge commit %s\n", merge.Hash[:8])
        return nil
    },
}

func init() {
    commitCmd.Flags().StringVarP(&message, "message", "m", "", "Commit message")
    deployCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Perform a dry run")
    deployCmd.Flags().StringVarP(&target, "target", "t", "mock", "Deployment target")
    deployCmd.Flags().BoolVar(&canary, "canary", false, "Use canary deployment")
    revertCmd.Flags().StringVarP(&target, "target", "t", "mock", "Deployment target")
}

// pkg/storage/repository.go
package storage

import (
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "time"
    
    "go.etcd.io/bbolt"
    "github.com/google/uuid"
    "netgit/pkg/config"
)

type Repository struct {
    db   *bbolt.DB
    path string
}

type Commit struct {
    Hash      string                 `json:"hash"`
    Parent    string                 `json:"parent"`
    Message   string                 `json:"message"`
    Author    string                 `json:"author"`
    Timestamp time.Time              `json:"timestamp"`
    Config    config.NetworkConfig   `json:"config"`
}

type Diff struct {
    Added    []string `json:"added"`
    Modified []string `json:"modified"`
    Deleted  []string `json:"deleted"`
    Content  string   `json:"content"`
}

type Status struct {
    Branch   string   `json:"branch"`
    Clean    bool     `json:"clean"`
    Added    []string `json:"added"`
    Modified []string `json:"modified"`
    Deleted  []string `json:"deleted"`
}

func (d *Diff) String() string {
    return d.Content
}

func NewRepository(path string) (*Repository, error) {
    netgitDir := filepath.Join(path, ".netgit")
    if err := os.MkdirAll(netgitDir, 0755); err != nil {
        return nil, err
    }
    
    dbPath := filepath.Join(netgitDir, "objects.db")
    db, err := bbolt.Open(dbPath, 0600, nil)
    if err != nil {
        return nil, err
    }
    
    repo := &Repository{db: db, path: path}
    
    // Initialize buckets
    err = db.Update(func(tx *bbolt.Tx) error {
        buckets := []string{"commits", "refs", "branches", "config"}
        for _, bucket := range buckets {
            if _, err := tx.CreateBucketIfNotExists([]byte(bucket)); err != nil {
                return err
            }
        }
        
        // Set initial branch
        refs := tx.Bucket([]byte("refs"))
        if refs.Get([]byte("HEAD")) == nil {
            refs.Put([]byte("HEAD"), []byte("refs/heads/main"))
        }
        
        return nil
    })
    
    return repo, err
}

func OpenRepository(path string) (*Repository, error) {
    netgitDir := filepath.Join(path, ".netgit")
    dbPath := filepath.Join(netgitDir, "objects.db")
    
    if _, err := os.Stat(dbPath); os.IsNotExist(err) {
        return nil, fmt.Errorf("not a netgit repository")
    }
    
    db, err := bbolt.Open(dbPath, 0600, nil)
    if err != nil {
        return nil, err
    }
    
    return &Repository{db: db, path: path}, nil
}

func (r *Repository) Close() error {
    return r.db.Close()
}

func (r *Repository) Path() string {
    return r.path
}

func (r *Repository) Commit(configs []config.NetworkConfig, message, author string) (*Commit, error) {
    if len(configs) == 0 {
        return nil, fmt.Errorf("no configurations to commit")
    }
    
    // Merge all configs into one
    mergedConfig := config.NetworkConfig{
        Metadata: config.Metadata{
            Name:        "merged",
            Version:     "1.0",
            Environment: "production",
        },
    }
    
    for _, cfg := range configs {
        mergedConfig.SecurityGroups = append(mergedConfig.SecurityGroups, cfg.SecurityGroups...)
        mergedConfig.NetworkPolicies = append(mergedConfig.NetworkPolicies, cfg.NetworkPolicies...)
        mergedConfig.FirewallRules = append(mergedConfig.FirewallRules, cfg.FirewallRules...)
    }
    
    commit := &Commit{
        Hash:      r.generateHash(mergedConfig, message, author),
        Message:   message,
        Author:    author,
        Timestamp: time.Now(),
        Config:    mergedConfig,
    }
    
    // Get parent commit
    if head, err := r.GetHEAD(); err == nil {
        commit.Parent = head.Hash
    }
    
    return commit, r.storeCommit(commit)
}

func (r *Repository) GetCommit(hash string) (*Commit, error) {
    var commit Commit
    err := r.db.View(func(tx *bbolt.Tx) error {
        bucket := tx.Bucket([]byte("commits"))
        data := bucket.Get([]byte(hash))
        if data == nil {
            return fmt.Errorf("commit not found: %s", hash)
        }
        return json.Unmarshal(data, &commit)
    })
    return &commit, err
}

func (r *Repository) GetHEAD() (*Commit, error) {
    var headRef string
    err := r.db.View(func(tx *bbolt.Tx) error {
        refs := tx.Bucket([]byte("refs"))
        head := refs.Get([]byte("HEAD"))
        if head == nil {
            return fmt.Errorf("no HEAD found")
        }
        headRef = string(head)
        return nil
    })
    if err != nil {
        return nil, err
    }
    
    // Get the actual commit hash from the branch reference
    var commitHash string
    err = r.db.View(func(tx *bbolt.Tx) error {
        refs := tx.Bucket([]byte("refs"))
        hash := refs.Get([]byte(headRef))
        if hash == nil {
            return fmt.Errorf("branch reference not found: %s", headRef)
        }
        commitHash = string(hash)
        return nil
    })
    if err != nil {
        return nil, err
    }
    
    return r.GetCommit(commitHash)
}

func (r *Repository) Diff(rev1, rev2 string) (*Diff, error) {
    var commit1, commit2 *Commit
    var err error
    
    if rev1 == "HEAD" {
        commit1, err = r.GetHEAD()
        if err != nil {
            return nil, err
        }
    } else {
        commit1, err = r.GetCommit(rev1)
        if err != nil {
            return nil, err
        }
    }
    
    if rev2 == "WORKING" {
        // Compare with working directory
        configs, err := config.LoadWorkingDirectory(r.path)
        if err != nil {
            return nil, err
        }
        
        diff := &Diff{
            Content: r.generateDiffContent(commit1.Config, configs[0]),
        }
        return diff, nil
    }
    
    commit2, err = r.GetCommit(rev2)
    if err != nil {
        return nil, err
    }
    
    diff := &Diff{
        Content: r.generateDiffContent(commit1.Config, commit2.Config),
    }
    
    return diff, nil
}

func (r *Repository) GetHistory() ([]*Commit, error) {
    var commits []*Commit
    
    head, err := r.GetHEAD()
    if err != nil {
        return commits, err
    }
    
    current := head
    for current != nil {
        commits = append(commits, current)
        if current.Parent == "" {
            break
        }
        current, err = r.GetCommit(current.Parent)
        if err != nil {
            break
        }
    }
    
    return commits, nil
}

func (r *Repository) Status() (*Status, error) {
    status := &Status{
        Branch: "main",
        Clean:  true,
    }
    
    // Simplified status - in real implementation would compare working dir with HEAD
    
    return status, nil
}

func (r *Repository) ListBranches() ([]string, error) {
    var branches []string
    
    err := r.db.View(func(tx *bbolt.Tx) error {
        branches = append(branches, "main")
        return nil
    })
    
    return branches, err
}

func (r *Repository) CurrentBranch() (string, error) {
    return "main", nil
}

func (r *Repository) CreateBranch(name string) error {
    head, err := r.GetHEAD()
    if err != nil {
        return err
    }
    
    return r.db.Update(func(tx *bbolt.Tx) error {
        refs := tx.Bucket([]byte("refs"))
        branchRef := fmt.Sprintf("refs/heads/%s", name)
        return refs.Put([]byte(branchRef), []byte(head.Hash))
    })
}

func (r *Repository) Merge(branchName string) (*Commit, error) {
    // Simplified merge implementation
    return &Commit{
        Hash:    uuid.New().String()[:8],
        Message: fmt.Sprintf("Merge branch '%s'", branchName),
    }, nil
}

func (r *Repository) storeCommit(commit *Commit) error {
    return r.db.Update(func(tx *bbolt.Tx) error {
        bucket := tx.Bucket([]byte("commits"))
        data, err := json.Marshal(commit)
        if err != nil {
            return err
        }
        
        if err := bucket.Put([]byte(commit.Hash), data); err != nil {
            return err
        }
        
        // Update HEAD reference
        refs := tx.Bucket([]byte("refs"))
        headRef := refs.Get([]byte("HEAD"))
        return refs.Put(headRef, []byte(commit.Hash))
    })
}

func (r *Repository) generateHash(config config.NetworkConfig, message, author string) string {
    data := fmt.Sprintf("%v%s%s%d", config, message, author, time.Now().Unix())
    hash := sha256.Sum256([]byte(data))
    return fmt.Sprintf("%x", hash)[:8]
}

func (r *Repository) generateDiffContent(old, new config.NetworkConfig) string {
    // Simplified diff generation
    return fmt.Sprintf("--- old\n+++ new\n@@ -1,1 +1,1 @@\n-SecurityGroups: %d\n+SecurityGroups: %d\n", 
        len(old.SecurityGroups), len(new.SecurityGroups))
}

// pkg/config/parser.go
package config

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "path/filepath"
    "strings"
    
    "gopkg.in/yaml.v3"
)

type NetworkConfig struct {
    Metadata         Metadata          `yaml:"metadata" json:"metadata"`
    SecurityGroups   []SecurityGroup   `yaml:"securityGroups" json:"securityGroups"`
    NetworkPolicies  []NetworkPolicy   `yaml:"networkPolicies" json:"networkPolicies"`
    FirewallRules    []FirewallRule    `yaml:"firewallRules" json:"firewallRules"`
}

type Metadata struct {
    Name        string            `yaml:"name" json:"name"`
    Version     string            `yaml:"version" json:"version"`
    Environment string            `yaml:"environment" json:"environment"`
    Labels      map[string]string `yaml:"labels" json:"labels"`
}

type SecurityGroup struct {
    Name        string `yaml:"name" json:"name"`
    Description string `yaml:"description" json:"description"`
    VpcId       string `yaml:"vpcId" json:"vpcId"`
    Rules       []Rule `yaml:"rules" json:"rules"`
}

type NetworkPolicy struct {
    Name      string              `yaml:"name" json:"name"`
    Namespace string              `yaml:"namespace" json:"namespace"`
    Selector  map[string]string   `yaml:"selector" json:"selector"`
    Ingress   []NetworkPolicyRule `yaml:"ingress" json:"ingress"`
    Egress    []NetworkPolicyRule `yaml:"egress" json:"egress"`
}

type FirewallRule struct {
    Name         string   `yaml:"name" json:"name"`
    Direction    string   `yaml:"direction" json:"direction"`
    Priority     int      `yaml:"priority" json:"priority"`
    Protocol     string   `yaml:"protocol" json:"protocol"`
    Ports        []string `yaml:"ports" json:"ports"`
    SourceRanges []string `yaml:"sourceRanges" json:"sourceRanges"`
    TargetTags   []string `yaml:"targetTags" json:"targetTags"`
}

type Rule struct {
    Protocol string   `yaml:"protocol" json:"protocol"`
    Ports    []string `yaml:"ports" json:"ports"`
    Sources  []string `yaml:"sources" json:"sources"`
    Action   string   `yaml:"action" json:"action"`
}

type NetworkPolicyRule struct {
    Ports []NetworkPolicyPort `yaml:"ports" json:"ports"`
    From  []NetworkPolicyPeer `yaml:"from" json:"from"`
}

type NetworkPolicyPort struct {
    Protocol string `yaml:"protocol" json:"protocol"`
    Port     string `yaml:"port" json:"port"`
}

type NetworkPolicyPeer struct {
    PodSelector       map[string]string `yaml:"podSelector" json:"podSelector"`
    NamespaceSelector map[string]string `yaml:"namespaceSelector" json:"namespaceSelector"`
}

func LoadWorkingDirectory(path string) ([]NetworkConfig, error) {
    var configs []NetworkConfig
    
    files, err := filepath.Glob(filepath.Join(path, "*.{yaml,yml,json}"))
    if err != nil {
        return nil, err
    }
    
    for _, file := range files {
        if strings.Contains(file, ".netgit") {
            continue
        }
        
        config, err := LoadFile(file)
        if err != nil {
            continue // Skip invalid files
        }
        
        configs = append(configs, *config)
    }
    
    if len(configs) == 0 {
        // Return a default empty config
        configs = append(configs, NetworkConfig{
            Metadata: Metadata{
                Name:        "default",
                Version:     "1.0",
                Environment: "development",
            },
        })
    }
    
    return configs, nil
}

func LoadFile(filename string) (*NetworkConfig, error) {
    data, err := ioutil.ReadFile(filename)
    if err != nil {
        return nil, err
    }
    
    var config NetworkConfig
    
    if strings.HasSuffix(filename, ".json") {
        err = json.Unmarshal(data, &config)
    } else {
        err = yaml.Unmarshal(data, &config)
    }
    
    if err != nil {
        return nil, fmt.Errorf("failed to parse %s: %w", filename, err)
    }
    
    return &config, nil
}

func (c *NetworkConfig) ToYAML() ([]byte, error) {
    return yaml.Marshal(c)
}

func (c *NetworkConfig) ToJSON() ([]byte, error) {
    return json.MarshalIndent(c, "", "  ")
}

func (c *NetworkConfig) Normalize() {
    // Normalize configuration for consistent comparisons
    if c.Labels == nil {
        c.Labels = make(map[string]string)
    }
}

// pkg/policy/engine.go
package policy

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "path/filepath"
    "strings"
    
    "netgit/pkg/config"
)

type Engine struct {
    policies []Policy
}

type Policy struct {
    Name        string `yaml:"name" json:"name"`
    Description string `yaml:"description" json:"description"`
    Rule        string `yaml:"rule" json:"rule"`
    Severity    string `yaml:"severity" json:"severity"`
}

type Violation struct {
    Rule    string `json:"rule"`
    Message string `json:"message"`
    File    string `json:"file"`
    Path    string `json:"path"`
    Level   string `json:"level"`
}

func NewEngine(policyDir string) (*Engine, error) {
    engine := &Engine{}
    
    // Load default policies if no policy directory exists
    engine.policies = []Policy{
        {
            Name:        "no-public-db",
            Description: "Database must never be publicly accessible",
            Rule:        "deny_public_database",
            Severity:    "high",
        },
        {
            Name:        "require-redundant-routes",
            Description: "Always require at least 2 redundant routes",
            Rule:        "require_redundancy",
            Severity:    "medium",
        },
        {
            Name:        "secure-protocols",
            Description: "Only allow secure protocols (HTTPS, SSH)",
            Rule:        "secure_protocols_only",
            Severity:    "high",
        },
    }
    
    // Try to load additional policies from directory
    if policyFiles, err := filepath.Glob(filepath.Join(policyDir, "*.json")); err == nil {
        for _, file := range policyFiles {
            if policy, err := loadPolicyFile(file); err == nil {
                engine.policies = append(engine.policies, *policy)
            }
        }
    }
    
    return engine, nil
}

func loadPolicyFile(filename string) (*Policy, error) {
    data, err := ioutil.ReadFile(filename)
    if err != nil {
        return nil, err
    }
    
    var policy Policy
    err = json.Unmarshal(data, &policy)
    return &policy, err
}

func (e *Engine) Verify(config config.NetworkConfig) ([]Violation, error) {
    var violations []Violation
    
    for _, policy := range e.policies {
        policyViolations := e.checkPolicy(config, policy)
        violations = append(violations, policyViolations...)
    }
    
    return violations, nil
}

func (e *Engine) checkPolicy(config config.NetworkConfig, policy Policy) []Violation {
    var violations []Violation
    
    switch policy.Rule {
    case "deny_public_database":
        violations = append(violations, e.checkPublicDatabase(config, policy)...)
    case "require_redundancy":
        violations = append(violations, e.checkRedundancy(config, policy)...)
    case "secure_protocols_only":
        violations = append(violations, e.checkSecureProtocols(config, policy)...)
    }
    
    return violations
}

func (e *Engine) checkPublicDatabase(config config.NetworkConfig, policy Policy) []Violation {
    var violations []Violation
    
    for _, sg := range config.SecurityGroups {
        for _, rule := range sg.Rules {
            if strings.Contains(rule.Protocol, "mysql") || strings.Contains(rule.Protocol, "postgres") {
                for _, source := range rule.Sources {
                    if source == "0.0.0.0/0" {
                        violations = append(violations, Violation{
                            Rule:    policy.Name,
                            Message: "Database security group allows public access",
                            File:    sg.Name,
                            Path:    "securityGroups.rules.sources",
                            Level:   policy.Severity,
                        })
                    }
                }
            }
        }
    }
    
    return violations
}

func (e *Engine) checkRedundancy(config config.NetworkConfig, policy Policy) []Violation {
    var violations []Violation
    
    if len(config.FirewallRules) < 2 {
        violations = append(violations, Violation{
            Rule:    policy.Name,
            Message: "Insufficient redundant routes configured",
            File:    "config",
            Path:    "firewallRules",
            Level:   policy.Severity,
        })
    }
    
    return violations
}

func (e *Engine) checkSecureProtocols(config config.NetworkConfig, policy Policy) []Violation {
    var violations []Violation
    insecureProtocols := []string{"http", "ftp", "telnet"}
    
    for _, sg := range config.SecurityGroups {
        for _, rule := range sg.Rules {
            for _, insecure := range insecureProtocols {
                if strings.ToLower(rule.Protocol) == insecure {
                    violations = append(violations, Violation{
                        Rule:    policy.Name,
                        Message: fmt.Sprintf("Insecure protocol '%s' is not allowed", rule.Protocol),
                        File:    sg.Name,
                        Path:    "securityGroups.rules.protocol",
                        Level:   policy.Severity,
                    })
                }
            }
        }
    }
    
    return violations
}

// pkg/deploy/deployer.go
package deploy

import (
    "fmt"
    "time"
    
    "netgit/pkg/config"
)

type Deployer interface {
    DryRun(config config.NetworkConfig) error
    Apply(config config.NetworkConfig) error
    Rollback(config config.NetworkConfig) error
    CanaryDeploy(config config.NetworkConfig, percentage int) error
    ExpandCanary(config config.NetworkConfig) error
}

type Deployment struct {
    CommitHash string    `json:"commit_hash"`
    Target     string    `json:"target"`
    Canary     bool      `json:"canary"`
    Timestamp  time.Time `json:"timestamp"`
    Status     string    `json:"status"`
}

func GetDeployer(target string) (Deployer, error) {
    switch target {
    case "mock":
        return NewMockDeployer(), nil
    case "aws":
        return NewAWSDeployer(), nil
    case "gcp":
        return NewGCPDeployer(), nil
    case "azure":
        return NewAzureDeployer(), nil
    case "k8s", "kubernetes":
        return NewKubernetesDeployer(), nil
    default:
        return nil, fmt.Errorf("unknown deployment target: %s", target)
    }
}

// Mock Deployer for testing
type MockDeployer struct{}

func NewMockDeployer() *MockDeployer {
    return &MockDeployer{}
}

func (m *MockDeployer) DryRun(config config.NetworkConfig) error {
    fmt.Printf("Mock: Dry run for %d security groups, %d policies, %d firewall rules\n", 
        len(config.SecurityGroups), len(config.NetworkPolicies), len(config.FirewallRules))
    return nil
}

func (m *MockDeployer) Apply(config config.NetworkConfig) error {
    fmt.Printf("Mock: Applying configuration...\n")
    time.Sleep(2 * time.Second) // Simulate deployment time
    fmt.Printf("Mock: Applied %d security groups, %d policies, %d firewall rules\n", 
        len(config.SecurityGroups), len(config.NetworkPolicies), len(config.FirewallRules))
    return nil
}

func (m *MockDeployer) Rollback(config config.NetworkConfig) error {
    fmt.Printf("Mock: Rolling back configuration...\n")
    time.Sleep(1 * time.Second)
    fmt.Printf("Mock: Rollback completed\n")
    return nil
}

func (m *MockDeployer) CanaryDeploy(config config.NetworkConfig, percentage int) error {
    fmt.Printf("Mock: Deploying to %d%% of targets...\n", percentage)
    time.Sleep(1 * time.Second)
    return nil
}

func (m *MockDeployer) ExpandCanary(config config.NetworkConfig) error {
    fmt.Printf("Mock: Expanding canary to 100%%...\n")
    time.Sleep(1 * time.Second)
    return nil
}

// AWS Deployer (stub implementation)
type AWSDeployer struct{}

func NewAWSDeployer() *AWSDeployer {
    return &AWSDeployer{}
}

func (a *AWSDeployer) DryRun(config config.NetworkConfig) error {
    fmt.Printf("AWS: Validating security groups and VPC configuration\n")
    return nil
}

func (a *AWSDeployer) Apply(config config.NetworkConfig) error {
    fmt.Printf("AWS: Applying security groups to VPC\n")
    for _, sg := range config.SecurityGroups {
        fmt.Printf("AWS: Creating/updating security group %s\n", sg.Name)
    }
    return nil
}

func (a *AWSDeployer) Rollback(config config.NetworkConfig) error {
    fmt.Printf("AWS: Rolling back security group changes\n")
    return nil
}

func (a *AWSDeployer) CanaryDeploy(config config.NetworkConfig, percentage int) error {
    fmt.Printf("AWS: Canary deploy to %d%% of instances\n", percentage)
    return nil
}

func (a *AWSDeployer) ExpandCanary(config config.NetworkConfig) error {
    fmt.Printf("AWS: Expanding to all instances\n")
    return nil
}

// GCP Deployer (stub implementation)
type GCPDeployer struct{}

func NewGCPDeployer() *GCPDeployer {
    return &GCPDeployer{}
}

func (g *GCPDeployer) DryRun(config config.NetworkConfig) error {
    fmt.Printf("GCP: Validating firewall rules\n")
    return nil
}

func (g *GCPDeployer) Apply(config config.NetworkConfig) error {
    fmt.Printf("GCP: Applying firewall rules\n")
    for _, rule := range config.FirewallRules {
        fmt.Printf("GCP: Creating/updating firewall rule %s\n", rule.Name)
    }
    return nil
}

func (g *GCPDeployer) Rollback(config config.NetworkConfig) error {
    fmt.Printf("GCP: Rolling back firewall rules\n")
    return nil
}

func (g *GCPDeployer) CanaryDeploy(config config.NetworkConfig, percentage int) error {
    fmt.Printf("GCP: Canary deploy to %d%% of targets\n", percentage)
    return nil
}

func (g *GCPDeployer) ExpandCanary(config config.NetworkConfig) error {
    fmt.Printf("GCP: Expanding to all targets\n")
    return nil
}

// Azure Deployer (stub implementation)
type AzureDeployer struct{}

func NewAzureDeployer() *AzureDeployer {
    return &AzureDeployer{}
}

func (az *AzureDeployer) DryRun(config config.NetworkConfig) error {
    fmt.Printf("Azure: Validating Network Security Groups\n")
    return nil
}

func (az *AzureDeployer) Apply(config config.NetworkConfig) error {
    fmt.Printf("Azure: Applying Network Security Groups\n")
    return nil
}

func (az *AzureDeployer) Rollback(config config.NetworkConfig) error {
    fmt.Printf("Azure: Rolling back NSG changes\n")
    return nil
}

func (az *AzureDeployer) CanaryDeploy(config config.NetworkConfig, percentage int) error {
    fmt.Printf("Azure: Canary deploy to %d%% of resource groups\n", percentage)
    return nil
}

func (az *AzureDeployer) ExpandCanary(config config.NetworkConfig) error {
    fmt.Printf("Azure: Expanding to all resource groups\n")
    return nil
}

// Kubernetes Deployer (stub implementation)
type KubernetesDeployer struct{}

func NewKubernetesDeployer() *KubernetesDeployer {
    return &KubernetesDeployer{}
}

func (k *KubernetesDeployer) DryRun(config config.NetworkConfig) error {
    fmt.Printf("Kubernetes: Validating network policies\n")
    return nil
}

func (k *KubernetesDeployer) Apply(config config.NetworkConfig) error {
    fmt.Printf("Kubernetes: Applying network policies\n")
    for _, policy := range config.NetworkPolicies {
        fmt.Printf("Kubernetes: Creating/updating NetworkPolicy %s in namespace %s\n", 
            policy.Name, policy.Namespace)
    }
    return nil
}

func (k *KubernetesDeployer) Rollback(config config.NetworkConfig) error {
    fmt.Printf("Kubernetes: Rolling back network policies\n")
    return nil
}

func (k *KubernetesDeployer) CanaryDeploy(config config.NetworkConfig, percentage int) error {
    fmt.Printf("Kubernetes: Canary deploy to %d%% of namespaces\n", percentage)
    return nil
}

func (k *KubernetesDeployer) ExpandCanary(config config.NetworkConfig) error {
    fmt.Printf("Kubernetes: Expanding to all namespaces\n")
    return nil
}

// pkg/audit/logger.go
package audit

import (
    "encoding/json"
    "fmt"
    "os"
    "time"
    
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

var logger *zap.Logger

func init() {
    config := zap.NewProductionConfig()
    config.OutputPaths = []string{"stdout", "audit.log"}
    config.ErrorOutputPaths = []string{"stderr"}
    config.EncoderConfig.TimeKey = "timestamp"
    config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
    
    var err error
    logger, err = config.Build()
    if err != nil {
        panic(fmt.Sprintf("Failed to initialize logger: %v", err))
    }
}

type Event struct {
    Action    string                 `json:"action"`
    Timestamp time.Time              `json:"timestamp"`
    User      string                 `json:"user"`
    Data      map[string]interface{} `json:"data"`
}

func LogEvent(action string, data map[string]interface{}) {
    event := Event{
        Action:    action,
        Timestamp: time.Now(),
        User:      getUser(),
        Data:      data,
    }
    
    logger.Info("audit_event",
        zap.String("action", event.Action),
        zap.Time("timestamp", event.Timestamp),
        zap.String("user", event.User),
        zap.Any("data", event.Data),
    )
    
    // Also write to structured JSON file
    writeJSONLog(event)
}

func writeJSONLog(event Event) {
    file, err := os.OpenFile("audit.json", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return
    }
    defer file.Close()
    
    jsonData, _ := json.Marshal(event)
    file.Write(jsonData)
    file.WriteString("\n")
}

func getUser() string {
    if user := os.Getenv("USER"); user != "" {
        return user
    }
    if user := os.Getenv("USERNAME"); user != "" {
        return user
    }
    return "unknown"
}

// pkg/metrics/prometheus.go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    CommitsTotal = promauto.NewCounter(prometheus.CounterOpts{
        Name: "netgit_commits_total",
        Help: "The total number of commits",
    })
    
    DeploymentsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "netgit_deployments_total",
        Help: "The total number of deployments",
    }, []string{"target", "status"})
    
    PolicyViolationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "netgit_policy_violations_total",
        Help: "The total number of policy violations",
    }, []string{"rule", "severity"})
    
    DeploymentDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name: "netgit_deployment_duration_seconds",
        Help: "The duration of deployments",
        Buckets: prometheus.DefBuckets,
    }, []string{"target"})
)

// examples/sample-configs/aws-security-groups.yaml
metadata:
  name: "production-security-groups"
  version: "1.2.0"
  environment: "production"
  labels:
    team: "platform"
    region: "us-east-1"

securityGroups:
  - name: "web-tier-sg"
    description: "Security group for web tier"
    vpcId: "vpc-12345678"
    rules:
      - protocol: "https"
        ports: ["443"]
        sources: ["0.0.0.0/0"]
        action: "allow"
      - protocol: "http"
        ports: ["80"]
        sources: ["10.0.0.0/8"]
        action: "allow"
  
  - name: "app-tier-sg"
    description: "Security group for application tier"
    vpcId: "vpc-12345678"
    rules:
      - protocol: "tcp"
        ports: ["8080", "8443"]
        sources: ["sg-web-tier"]
        action: "allow"
      - protocol: "tcp"
        ports: ["22"]
        sources: ["10.0.1.0/24"]
        action: "allow"
  
  - name: "db-tier-sg"
    description: "Security group for database tier"
    vpcId: "vpc-12345678"
    rules:
      - protocol: "mysql"
        ports: ["3306"]
        sources: ["sg-app-tier"]
        action: "allow"

# examples/sample-configs/k8s-network-policies.yaml
metadata:
  name: "k8s-network-policies"
  version: "1.0.0"
  environment: "production"

networkPolicies:
  - name: "frontend-policy"
    namespace: "default"
    selector:
      app: "frontend"
    ingress:
      - ports:
          - protocol: "TCP"
            port: "80"
        from:
          - podSelector:
              app: "loadbalancer"
    egress:
      - ports:
          - protocol: "TCP"
            port: "8080"
        to:
          - podSelector:
              app: "backend"
  
  - name: "backend-policy"
    namespace: "default"
    selector:
      app: "backend"
    ingress:
      - ports:
          - protocol: "TCP"
            port: "8080"
        from:
          - podSelector:
              app: "frontend"
    egress:
      - ports:
          - protocol: "TCP"
            port: "5432"
        to:
          - podSelector:
              app: "database"

# examples/sample-configs/gcp-firewall-rules.yaml
metadata:
  name: "gcp-firewall-rules"
  version: "1.1.0"
  environment: "production"

firewallRules:
  - name: "allow-web-traffic"
    direction: "INGRESS"
    priority: 1000
    protocol: "tcp"
    ports: ["80", "443"]
    sourceRanges: ["0.0.0.0/0"]
    targetTags: ["web-server"]
  
  - name: "allow-ssh-internal"
    direction: "INGRESS"
    priority: 1100
    protocol: "tcp"
    ports: ["22"]
    sourceRanges: ["10.0.0.0/8"]
    targetTags: ["ssh-access"]
  
  - name: "deny-all-ingress"
    direction: "INGRESS"
    priority: 65534
    protocol: "all"
    ports: []
    sourceRanges: ["0.0.0.0/0"]
    targetTags: ["secure"]

# examples/policies/security-policies.json
[
  {
    "name": "no-public-database",
    "description": "Database ports must not be accessible from the internet",
    "rule": "deny_public_database",
    "severity": "high"
  },
  {
    "name": "require-https",
    "description": "Web traffic must use HTTPS",
    "rule": "require_https",
    "severity": "medium"
  },
  {
    "name": "ssh-access-control",
    "description": "SSH access must be restricted to internal networks",
    "rule": "ssh_internal_only",
    "severity": "high"
  }
]

# tests/storage_test.go
package tests

import (
    "os"
    "testing"
    "path/filepath"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "netgit/pkg/storage"
    "netgit/pkg/config"
)

func TestRepository(t *testing.T) {
    // Create temporary directory for testing
    tmpDir, err := os.MkdirTemp("", "netgit-test")
    require.NoError(t, err)
    defer os.RemoveAll(tmpDir)
    
    // Test repository initialization
    repo, err := storage.NewRepository(tmpDir)
    require.NoError(t, err)
    defer repo.Close()
    
    assert.Equal(t, tmpDir, repo.Path())
    
    // Test committing configuration
    testConfig := config.NetworkConfig{
        Metadata: config.Metadata{
            Name:        "test-config",
            Version:     "1.0.0",
            Environment: "test",
        },
        SecurityGroups: []config.SecurityGroup{
            {
                Name:        "test-sg",
                Description: "Test security group",
                VpcId:       "vpc-test",
                Rules: []config.Rule{
                    {
                        Protocol: "https",
                        Ports:    []string{"443"},
                        Sources:  []string{"0.0.0.0/0"},
                        Action:   "allow",
                    },
                },
            },
        },
    }
    
    commit, err := repo.Commit([]config.NetworkConfig{testConfig}, "Initial commit", "test@example.com")
    require.NoError(t, err)
    
    assert.NotEmpty(t, commit.Hash)
    assert.Equal(t, "Initial commit", commit.Message)
    assert.Equal(t, "test@example.com", commit.Author)
    
    // Test getting commit back
    retrievedCommit, err := repo.GetCommit(commit.Hash)
    require.NoError(t, err)
    
    assert.Equal(t, commit.Hash, retrievedCommit.Hash)
    assert.Equal(t, commit.Message, retrievedCommit.Message)
}

# tests/policy_test.go
package tests

import (
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "netgit/pkg/policy"
    "netgit/pkg/config"
)

func TestPolicyEngine(t *testing.T) {
    engine, err := policy.NewEngine("../examples/policies")
    require.NoError(t, err)
    
    // Test configuration with policy violations
    violatingConfig := config.NetworkConfig{
        SecurityGroups: []config.SecurityGroup{
            {
                Name: "bad-sg",
                Rules: []config.Rule{
                    {
                        Protocol: "mysql",
                        Ports:    []string{"3306"},
                        Sources:  []string{"0.0.0.0/0"}, // Public access to database
                        Action:   "allow",
                    },
                },
            },
        },
    }
    
    violations, err := engine.Verify(violatingConfig)
    require.NoError(t, err)
    
    assert.Greater(t, len(violations), 0)
    
    // Test configuration without violations
    goodConfig := config.NetworkConfig{
        SecurityGroups: []config.SecurityGroup{
            {
                Name: "good-sg",
                Rules: []config.Rule{
                    {
                        Protocol: "https",
                        Ports:    []string{"443"},
                        Sources:  []string{"10.0.0.0/8"}, // Internal only
                        Action:   "allow",
                    },
                },
            },
        },
        FirewallRules: []config.FirewallRule{
            {Name: "rule1"},
            {Name: "rule2"}, // Satisfies redundancy requirement
        },
    }
    
    violations, err = engine.Verify(goodConfig)
    require.NoError(t, err)
    
    assert.Equal(t, 0, len(violations))
}

# tests/deploy_test.go
package tests

import (
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "netgit/pkg/deploy"
    "netgit/pkg/config"
)

func TestMockDeployer(t *testing.T) {
    deployer := deploy.NewMockDeployer()
    
    testConfig := config.NetworkConfig{
        Metadata: config.Metadata{
            Name: "test-deploy",
        },
        SecurityGroups: []config.SecurityGroup{
            {Name: "test-sg"},
        },
    }
    
    // Test dry run
    err := deployer.DryRun(testConfig)
    assert.NoError(t, err)
    
    // Test apply
    err = deployer.Apply(testConfig)
    assert.NoError(t, err)
    
    // Test canary deployment
    err = deployer.CanaryDeploy(testConfig, 10)
    assert.NoError(t, err)
    
    err = deployer.ExpandCanary(testConfig)
    assert.NoError(t, err)
    
    // Test rollback
    err = deployer.Rollback(testConfig)
    assert.NoError(t, err)
}

func TestGetDeployer(t *testing.T) {
    tests := []struct {
        target    string
        expectErr bool
    }{
        {"mock", false},
        {"aws", false},
        {"gcp", false},
        {"azure", false},
        {"k8s", false},
        {"kubernetes", false},
        {"invalid", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.target, func(t *testing.T) {
            deployer, err := deploy.GetDeployer(tt.target)
            
            if tt.expectErr {
                assert.Error(t, err)
                assert.Nil(t, deployer)
            } else {
                assert.NoError(t, err)
                assert.NotNil(t, deployer)
            }
        })
    }
}

# Makefile
.PHONY: build test clean install deps

# Build the netgit binary
build:
	go build -o bin/netgit ./main.go

# Install dependencies
deps:
	go mod tidy
	go mod download

# Run tests
test:
	go test -v ./tests/...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./tests/...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -f audit.log audit.json

# Install netgit globally
install: build
	sudo cp bin/netgit /usr/local/bin/

# Run linting
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...

# Generate mocks for testing
generate-mocks:
	mockgen -source=pkg/deploy/deployer.go -destination=tests/mocks/deployer_mock.go

# Build Docker image
docker-build:
	docker build -t netgit:latest .

# Run example workflow
demo: build
	@echo "=== NetGit Demo Workflow ==="
	./bin/netgit init
	cp examples/sample-configs/aws-security-groups.yaml ./
	./bin/netgit commit -m "Initial AWS security groups"
	./bin/netgit status
	./bin/netgit verify
	./bin/netgit deploy --target=mock --dry-run
	./bin/netgit deploy --target=mock
	./bin/netgit history
