# Diagram Catalogue

All 34 project diagrams. Each one exists in three forms:

| Folder | Contents | Use it for |
|---|---|---|
| [`src/`](src/) | Mermaid source (`.mmd`), the **editable master** | Changing a diagram (in the same pull request as the design change) |
| [`svg/`](svg/) | Vector images | Docs, web pages, zooming |
| [`png/`](png/) | Raster images (2× resolution) | Word/PowerPoint, chat, printing |

**Regenerate** after editing a source file (requires Node.js and Chrome or Edge):

```bash
npx -p @mermaid-js/mermaid-cli@11.4.2 mmdc -i src/<name>.mmd -o svg/<name>.svg -b white
npx -p @mermaid-js/mermaid-cli@11.4.2 mmdc -i src/<name>.mmd -o png/<name>.png -b white -s 2
```

(From Phase 1, `go run ./tools/devtool diagrams` regenerates all of them.)

## Notation

| Diagram type | Standard | Notation notes |
|---|---|---|
| Use case (UC) | UML 2.5 | Actors on the left (👤 human, ⚙ «system»), rounded ovals = use cases, box = system boundary, dotted «include» / «extend» / «generalization» |
| Class (CD) | UML 2.5 | `+` public, `-` private; ◆ composition, → association, ▷ inheritance/realisation; «stereotype» names the Go package |
| Sequence (SD) | UML 2.5 | `alt` / `loop` / `par` combined fragments; numbered messages |
| Activity (AD) | UML 2.5 | ● start, ◉ end, ◇ decision, {{fork/join}} |
| State machine (ST) | UML 2.5 | `[guard]` conditions on transitions |
| Component (CP) | UML 2.5 | «component», «layer», «executable», «database» stereotypes |
| Deployment (DP) | UML 2.5 | «device» nodes containing «artifact»s and «execution environment»s |
| Entity-relationship (ER) | Crow's foot | PK / FK / UK markers |
| Context (CX) | C4 model, level 1 | |
| Project (PM) | Gantt, WBS | |

## 1. Context and Use Cases

### CX-01 System Context
![CX-01](png/CX-01-system-context.png)

### UC-01 System Use Case Overview
![UC-01](png/UC-01-system-overview.png)

### UC-02 Code Hosting and Collaboration
![UC-02](png/UC-02-code-collaboration.png)

### UC-03 CI/CD and Publishing
![UC-03](png/UC-03-cicd-publishing.png)

### UC-04 Administration and Insight
![UC-04](png/UC-04-administration.png)

## 2. Structure

### CP-01 Server Components
![CP-01](png/CP-01-server-components.png)

### CP-02 Runner Components
![CP-02](png/CP-02-runner-components.png)

### CD-01 Domain Model
![CD-01](png/CD-01-domain-model.png)

### CD-02 Git Hosting Subsystem
![CD-02](png/CD-02-git-subsystem.png)

### CD-03 CI/CD Engine (Server)
![CD-03](png/CD-03-ci-subsystem.png)

### CD-04 Runner
![CD-04](png/CD-04-runner.png)

### CD-05 Authentication, Authorization and Secrets
![CD-05](png/CD-05-security.png)

### ER-01 Database Schema
![ER-01](png/ER-01-database.png)

## 3. Behaviour — Sequence Diagrams

### SD-01 Sign In
![SD-01](png/SD-01-login.png)

### SD-02 Clone / Fetch
![SD-02](png/SD-02-clone.png)

### SD-03 Push with Server Hooks
![SD-03](png/SD-03-push.png)

### SD-04 Open Pull Request
![SD-04](png/SD-04-open-pull-request.png)

### SD-05 Merge Pull Request
![SD-05](png/SD-05-merge-pull-request.png)

### SD-06 Pipeline Trigger
![SD-06](png/SD-06-pipeline-trigger.png)

### SD-07 Job Execution
![SD-07](png/SD-07-job-execution.png)

### SD-08 Release and Deploy
![SD-08](png/SD-08-release-and-deploy.png)

### SD-09 Cancel a Job
![SD-09](png/SD-09-cancel-job.png)

## 4. Behaviour — Activity Diagrams

### AD-01 Push Processing
![AD-01](png/AD-01-push-processing.png)

### AD-02 Trigger Evaluation
![AD-02](png/AD-02-trigger-evaluation.png)

### AD-03 Scheduler Advance
![AD-03](png/AD-03-scheduler-advance.png)

### AD-04 Runner Job Execution
![AD-04](png/AD-04-runner-job.png)

### AD-05 Merge Pull Request
![AD-05](png/AD-05-merge-pull-request.png)

## 5. Behaviour — State Machines

### ST-01 Job Lifecycle
![ST-01](png/ST-01-job-states.png)

### ST-02 Pull Request Lifecycle
![ST-02](png/ST-02-pull-request-states.png)

### ST-03 Runner Lifecycle
![ST-03](png/ST-03-runner-states.png)

## 6. Deployment

### DP-01 Single Machine
![DP-01](png/DP-01-single-server.png)

### DP-02 Team / Production
![DP-02](png/DP-02-team-production.png)

## 7. Project Management

### PM-01 Schedule
![PM-01](png/PM-01-schedule-gantt.png)

### PM-02 Work Breakdown Structure
![PM-02](png/PM-02-work-breakdown-structure.png)
