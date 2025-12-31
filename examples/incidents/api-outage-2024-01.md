# Incident Post-Mortem: API Service Outage

## Incident ID: INC-2024-001
**Date**: January 15, 2024
**Duration**: 45 minutes
**Severity**: SEV1

## Summary
The API service experienced a complete outage for 45 minutes due to database connection pool exhaustion caused by a slow query introduced in a recent deployment.

## Timeline

| Time (UTC) | Event |
|------------|-------|
| 14:00 | Deployment of version 2.3.1 completed |
| 14:15 | First alerts for increased API latency |
| 14:20 | Connection pool exhausted errors appearing |
| 14:25 | API service becomes completely unavailable |
| 14:30 | Incident declared, on-call engineer engaged |
| 14:35 | Root cause identified: slow query in new feature |
| 14:40 | Rollback initiated |
| 14:45 | Service restored after rollback |
| 15:00 | Full recovery confirmed |

## Root Cause

A new feature introduced in version 2.3.1 included a database query that performed a full table scan on the `user_activities` table (50M+ rows) without proper indexing. Under production load, this query took 30+ seconds to complete, holding database connections and eventually exhausting the connection pool.

### Contributing Factors
1. Missing index on `user_activities.created_at` column
2. Query not load tested before deployment
3. Connection pool timeout set to 60 seconds (too high)
4. No circuit breaker for database operations

## Impact

- **Users affected**: ~50,000
- **Failed requests**: ~1.2 million
- **Revenue impact**: Estimated $15,000 in lost transactions
- **SLA violation**: 99.9% monthly SLA breached

## Resolution

### Immediate Actions
1. Rolled back to version 2.3.0
2. Restarted API service pods to clear connection pool
3. Verified service recovery

### Follow-up Actions
1. Added index on `user_activities.created_at`
2. Optimized query to use pagination
3. Reduced connection pool timeout to 10 seconds
4. Added circuit breaker for database operations

## Lessons Learned

### What went well
- Alerts fired within 15 minutes of issue starting
- On-call response was quick
- Rollback procedure worked smoothly

### What could be improved
- Query performance testing in staging
- Load testing for new features
- Better monitoring of connection pool metrics

## Action Items

| Action | Owner | Due Date | Status |
|--------|-------|----------|--------|
| Add query performance testing to CI/CD | Platform Team | Jan 30 | Done |
| Implement connection pool monitoring dashboard | SRE | Jan 25 | Done |
| Add database circuit breaker | API Team | Feb 1 | Done |
| Update deployment checklist | Engineering | Jan 20 | Done |
| Conduct slow query training session | DB Team | Feb 15 | Done |

## Related
- Runbook: Database Connection Pool Exhaustion
- Runbook: API Service Rollback Procedure

## Tags
- database
- connection-pool
- deployment
- rollback
- slow-query
