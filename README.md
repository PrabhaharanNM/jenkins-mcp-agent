# MCP Agent: AI-Powered Build Failure Analysis

A Jenkins plugin that automatically analyzes build failures using AI (Claude via AWS Bedrock). It runs 4 MCP agents in parallel to gather context from Jenkins, BitBucket, Kubernetes, and JFrog, then generates comprehensive root cause analysis reports.

## Architecture

**Hybrid Java + Go design:**
- **Java layer** (~500 lines): Thin Jenkins plugin wrapper for UI, configuration, and subprocess management
- **Go binary** (~3000 lines): High-performance core for AI processing, parallel MCP agents, and report generation

```
Jenkins Build Fails
       │
       ▼
Java Plugin (post-build action)
       │
       ▼ subprocess call
Go Binary
  ├── Fetch console logs
  ├── Parse errors, stages, repos
  ├── Run 4 MCP agents in parallel:
  │   ├── Jenkins  (stages, agent metrics)
  │   ├── BitBucket (commits, CODEOWNERS)
  │   ├── Kubernetes (pods, OOM, events)
  │   └── JFrog (artifact availability)
  ├── Cross-correlate findings
  ├── Claude AI analysis (AWS Bedrock)
  ├── Generate HTML report
  ├── Create Jira ticket (async)
  └── Send email notification (async)
```

## Performance

| Metric | Value |
|--------|-------|
| Memory | 20-30 MB per analysis |
| Startup | <100 ms |
| Analysis time | 8-20 seconds |
| Concurrent analyses | 10+ |
| Plugin size | ~40 MB (all platforms) |

## Installation

1. Download `mcp-agent.hpi` from releases
2. Go to **Manage Jenkins** -> **Manage Plugins** -> **Advanced**
3. Upload the `.hpi` file under "Deploy Plugin"
4. Restart Jenkins

## Configuration

Go to **Manage Jenkins** -> **Configure System** -> **MCP Agent** section.

### Required Settings

- **AWS Bedrock**: Region, Model ID, AWS profile (for authentication)
- **BitBucket**: Server URL, credentials
- **Jira**: Server URL, credentials, project key

### Optional Settings

- **Kubernetes**: API server URL, auth token, namespace
- **JFrog**: Artifactory URL, credentials
- **Email/SMTP**: Mail server settings for notifications
- **Team Mappings**: JSON mapping repository names to team managers

### Team Mappings Example

```json
{
  "payments": {"name": "Payments Team Lead", "email": "payments-lead@example.com", "jiraUsername": "payments-lead"},
  "orders": {"name": "Orders Team Lead", "email": "orders-lead@example.com", "jiraUsername": "orders-lead"}
}
```

## Usage

### Declarative Pipeline

```groovy
pipeline {
    agent any
    stages {
        stage('Build') {
            steps { sh 'mvn clean package' }
        }
    }
    post {
        failure {
            mcpAnalyze()
        }
    }
}
```

### With Repository Configuration

```groovy
post {
    failure {
        mcpAnalyze(
            repositories: [
                [name: 'payments', workspace: 'myproject', branch: env.GIT_BRANCH],
                [name: 'orders', workspace: 'myproject', branch: env.GIT_BRANCH]
            ]
        )
    }
}
```

### Freestyle Project

1. Open job configuration
2. Add post-build action: **MCP Agent: AI-Powered Failure Analysis**
3. Optionally configure repository list
4. Save

## What You Get

When a build fails:

1. **Console output** shows analysis ID and results URL
2. **Sidebar link** "AI Analysis" appears on the build page
3. **Analysis report** with:
   - Failure category and root cause summary
   - Detailed explanation with evidence
   - Responsible team identification
   - Actionable next steps
4. **Jira ticket** created and assigned to the responsible team
5. **Email notification** sent with the HTML report

## Building from Source

### Prerequisites

- JDK 11+
- Maven 3.8+
- Go 1.21+

### Build

```bash
# Build Go binaries for all platforms
cd go-binary
./build.sh

# Build Jenkins plugin
cd ..
mvn clean package
```

The `.hpi` file will be in `target/mcp-agent.hpi`.

## Troubleshooting

- **"Go binary not found"**: Ensure binaries are in `src/main/resources/binaries/`
- **Analysis stuck on "pending"**: Check Jenkins system logs for Go binary errors
- **AWS auth fails**: Verify AWS profile or instance role configuration
- **Jira ticket not created**: Check Jira credentials and project key in global config

## License

MIT License
