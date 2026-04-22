# MLOps Scenarios — 20 Real-World Situations

Ordered by prerequisite dependency. Difficulty rating: 1 (easiest) → 5 (hardest).
Each scenario includes the failure mode you are most likely to be asked about in an interview.

---

## Branch: Data Engineering for ML

### Scenario 1 — Data Pipeline Ingestion Failure [Difficulty: 1]
A batch job that moves raw data from S3 into a training dataset fails silently. The model trains on stale data from 3 days ago without anyone noticing until prediction quality drops.
**Key concepts:** pipeline stages (ingest → validate → transform → store), idempotency, data freshness SLAs.
**Failure mode:** no schema validation at ingestion; downstream model trains on wrong window.

### Scenario 2 — Feature Store Mismatch [Difficulty: 2]
The online feature store (serving real-time predictions) computes features differently from the offline feature store (used in training). The model was trained on 7-day rolling averages; production serves 1-day averages.
**Key concepts:** online vs offline stores, point-in-time correctness, training-serving skew.
**Failure mode:** model accuracy looks fine in offline eval but degrades immediately in production.

### Scenario 3 — Input Distribution Shift [Difficulty: 3]
A fraud detection model starts flagging 40% more transactions as fraudulent after a new mobile app version ships. The new app sends slightly different device fingerprint fields.
**Key concepts:** covariate shift, PSI (Population Stability Index), feature-level drift detection.
**Failure mode:** monitoring only tracked prediction output, not input distribution — drift was invisible.

### Scenario 4 — Dataset Version Conflict [Difficulty: 2]
A model trained on dataset v1.3 is deployed. A bug is found and a rollback is needed, but v1.3 was overwritten by v1.4 in the data lake. Retraining is impossible without reconstruction.
**Key concepts:** data versioning (DVC, Delta Lake snapshots), immutable datasets, data lineage.
**Failure mode:** no version pinning on training artifacts; rollback blocked.

---

## Branch: Model Lifecycle

### Scenario 5 — Unstable Training Run [Difficulty: 2]
A training job runs for 6 hours and then diverges — loss goes to NaN at epoch 47. The team cannot reproduce it because learning rate and batch size were changed mid-run without logging.
**Key concepts:** training loop stability, gradient clipping, experiment logging (MLflow/W&B), checkpoint saving.
**Failure mode:** no experiment tracking; the failing config is unrecoverable.

### Scenario 6 — Overfitting to Validation Set [Difficulty: 3]
After 30 hyperparameter tuning runs, the best model achieves 94% validation accuracy but only 71% on held-out test data. The team used the test set to pick the final model.
**Key concepts:** train/val/test discipline, data leakage via hyperparameter selection, nested cross-validation.
**Failure mode:** test set contamination — it was used as a second validation set.

### Scenario 7 — Wrong Metric for Business Problem [Difficulty: 3]
A recommendation model achieves 0.92 AUC. After deployment, click-through rate drops 12%. The model learned to predict what users *had* clicked, not what they *would* click next.
**Key concepts:** offline metric selection, precision@K vs AUC, proxy metric traps, business KPI alignment.
**Failure mode:** AUC measures discrimination, not ranking quality at the top — wrong metric for recommendations.

### Scenario 8 — Hyperparameter Search at Scale [Difficulty: 3]
A team runs grid search over 5 hyperparameters with 4 values each — 1,024 runs. It takes 3 weeks and the cloud bill exceeds budget. The best model is marginally better than the baseline.
**Key concepts:** Bayesian optimisation, early stopping, successive halving (Hyperband), compute cost tradeoffs.
**Failure mode:** grid search scales exponentially; Bayesian search would have found a good result in ~50 runs.

---

## Branch: Deployment & Serving

### Scenario 9 — Training-Serving Skew [Difficulty: 3]
A model performs well in offline eval. In production, predictions are consistently wrong for one user segment. Investigation reveals the serving code applies feature normalisation differently from the training pipeline.
**Key concepts:** model packaging, input preprocessing contracts, ONNX/TorchScript serialisation, schema enforcement at serving time.
**Failure mode:** preprocessing logic duplicated in two places; they diverged silently.

### Scenario 10 — Model Server Latency Spike [Difficulty: 3]
A recommendation API has a p99 latency of 800ms under load. SLA is 200ms. The model is a 300M parameter transformer.
**Key concepts:** model quantisation (INT8), batching strategies, GPU vs CPU serving, TensorRT, async inference.
**Failure mode:** model too large for latency requirements; no batching configured.

### Scenario 11 — Canary Deployment Gone Wrong [Difficulty: 4]
A new model is deployed to 10% of traffic. Error rate stays flat but a business KPI (conversion) drops 8% for the canary group. The team doesn't catch it because their rollout dashboard only shows error rate.
**Key concepts:** canary vs A/B test, multi-metric rollout gates, statistical significance, automatic rollback triggers.
**Failure mode:** monitoring only system metrics, not business metrics, during rollout.

### Scenario 12 — Shadow Mode Comparison [Difficulty: 4]
Before replacing a production model, a team runs the new model in shadow mode — it receives the same requests but its predictions are not served. They discover the new model has 3× higher latency but 15% better accuracy.
**Key concepts:** shadow mode deployment, champion-challenger pattern, latency-accuracy tradeoff decisions.
**Failure mode:** accuracy alone was the success criterion; latency impact was not evaluated pre-deployment.

---

## Branch: Monitoring & Reliability

### Scenario 13 — Silent Model Degradation [Difficulty: 4]
A churn prediction model has been in production for 8 months. No one has looked at it since deployment. A quarterly review reveals prediction accuracy fell from 84% to 61% six months ago, aligned with a CRM schema change.
**Key concepts:** production monitoring, prediction distribution tracking, alert thresholds, upstream dependency monitoring.
**Failure mode:** no monitoring; degradation was invisible for months.

### Scenario 14 — Slice-Level Failure [Difficulty: 4]
An aggregate accuracy metric looks healthy. A customer complaint reveals the model fails specifically for users on iOS 17 with Spanish locale — a slice representing 3% of users.
**Key concepts:** slice-based evaluation, disaggregated metrics, fairness monitoring, cohort analysis.
**Failure mode:** aggregate metrics masked the slice failure; no per-cohort monitoring.

### Scenario 15 — Automated Retraining Instability [Difficulty: 5]
A daily retraining pipeline triggers on data volume. One day a data pipeline bug sends 10× normal volume with corrupted labels. The model retrains on bad data and is automatically promoted to production.
**Key concepts:** retraining triggers, model quality gates before promotion, data validation pre-train, human-in-the-loop gates.
**Failure mode:** automated promotion with no quality gate; corrupt model went live.

---

## Branch: MLOps Infrastructure

### Scenario 16 — Environment Inconsistency [Difficulty: 2]
A model trains successfully on a researcher's laptop (CUDA 11.8, PyTorch 2.0). The Docker image on the training cluster uses CUDA 12.1 and PyTorch 2.1. Behaviour differences cause non-deterministic results.
**Key concepts:** Docker for ML, CUDA/library version pinning, reproducible environments, multi-stage builds.
**Failure mode:** environment not pinned; implicit dependency on OS-level CUDA driver version.

### Scenario 17 — Pipeline DAG Failure Recovery [Difficulty: 3]
A 4-stage training pipeline (ingest → featurise → train → evaluate) fails at the evaluate stage after 5 hours. The team re-runs from the start, spending another 5 hours on already-completed stages.
**Key concepts:** DAG orchestration (Airflow/Prefect), task-level checkpointing, idempotent stages, failure recovery strategies.
**Failure mode:** no intermediate artifact caching; rerun always starts from scratch.

### Scenario 18 — Model Registry Promotion Conflict [Difficulty: 3]
Two teams simultaneously train models for the same use case. Both pass staging gates and are promoted to production within minutes of each other. The second promotion overwrites the first.
**Key concepts:** model registry (MLflow, SageMaker), staging → production gates, optimistic locking, lineage tracking.
**Failure mode:** no concurrency control on registry promotion.

### Scenario 19 — GPU Cost Spike [Difficulty: 4]
A model training job is accidentally run with 16 A100s instead of 4 (a config file typo). It runs for 12 hours before anyone notices. Cloud bill for that day is 40× normal.
**Key concepts:** compute resource management, spot vs reserved instances, budget alerts, right-sizing, auto-scaling.
**Failure mode:** no spend alert; resource config not validated before job submission.

### Scenario 20 — End-to-End System Design Under Constraints [Difficulty: 5]
You are asked to design an ML platform for a startup: a recommendation engine that must serve 10,000 requests/second at p99 < 150ms, retrain weekly, cost under $5,000/month, and have zero manual steps in the retraining loop.
**Key concepts:** full pipeline design, cost-latency-freshness triangle, orchestration choice, serving infrastructure, monitoring strategy, build vs buy decisions.
**Failure mode:** This is a design question — the failure is in the reasoning: optimising one axis (e.g. freshness) while ignoring another (e.g. cost or latency).
