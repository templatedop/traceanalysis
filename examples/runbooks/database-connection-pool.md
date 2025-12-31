# Runbook: Database Connection Pool Exhaustion

## Overview
This runbook provides guidance for investigating and resolving database connection pool exhaustion issues.

## Symptoms
- "Connection pool exhausted" errors in logs
- Timeouts when acquiring database connections
- Increased latency in database operations
- Service degradation or complete failure

## Investigation Steps

### 1. Check connection pool metrics
```promql
# Active connections
db_pool_active_connections{service="$service"}

# Pool size
db_pool_size{service="$service"}

# Wait time for connections
histogram_quantile(0.95, rate(db_pool_acquire_duration_seconds_bucket[5m]))
```

### 2. Identify connection leaks
Check for:
- Connections not being returned to pool
- Long-running transactions
- Unclosed connections in error paths

### 3. Analyze query patterns
```sql
-- PostgreSQL: Show current connections
SELECT
    pid,
    usename,
    application_name,
    client_addr,
    state,
    query_start,
    query
FROM pg_stat_activity
WHERE datname = 'your_database';

-- MySQL: Show processlist
SHOW FULL PROCESSLIST;
```

### 4. Check for slow queries
```sql
-- PostgreSQL: Slow queries
SELECT
    query,
    calls,
    mean_exec_time,
    total_exec_time
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;
```

## Common Causes

### Connection Leaks
- Connections not returned in error paths
- Missing defer/finally blocks
- ORM session management issues

### Pool Sizing
- Pool too small for traffic
- Incorrect max connection settings
- Missing connection timeout

### Slow Queries
- Missing indexes
- N+1 query patterns
- Large result sets

### External Factors
- Database server overload
- Network issues
- DNS resolution problems

## Resolution Steps

### Immediate Actions
1. Restart affected service (temporary fix)
2. Scale up database if possible
3. Kill long-running queries

### Short-term
1. Increase connection pool size (with caution)
2. Add connection timeout
3. Enable connection pool metrics

### Long-term
1. Fix connection leaks
2. Optimize slow queries
3. Implement connection pooler (PgBouncer, ProxySQL)
4. Review and optimize pool settings

## Recommended Pool Settings

```yaml
# Example configuration
database:
  pool:
    max_connections: 20
    min_connections: 5
    max_idle_time: 10m
    acquire_timeout: 30s
    health_check_interval: 30s
```

## Prevention
- Monitor connection pool usage
- Set up alerts for pool utilization > 80%
- Regular query performance reviews
- Connection leak detection in CI/CD

## Related
- Runbook: Database Performance Issues
- Runbook: N+1 Query Detection
