# Runbook: High CPU Usage

## Overview
This runbook provides guidance for investigating and resolving high CPU usage issues in services.

## Symptoms
- CPU usage consistently above 80%
- Increased latency in service responses
- Service becoming unresponsive
- Alerts triggered for CPU thresholds

## Investigation Steps

### 1. Identify the affected service
```bash
# Check CPU usage by container
kubectl top pods --sort-by=cpu

# Or using Prometheus
# Query: sum(rate(container_cpu_usage_seconds_total[5m])) by (container) * 100
```

### 2. Check for recent changes
- Review recent deployments
- Check for configuration changes
- Review traffic patterns

### 3. Profile the application
For Go services:
```bash
# Enable pprof endpoint and collect CPU profile
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
```

For Java services:
```bash
# Use async-profiler or JFR
java -XX:+FlightRecorder -XX:StartFlightRecording=duration=60s,filename=cpu.jfr
```

### 4. Common causes

#### GC Pressure
- **Symptoms**: Frequent GC pauses, heap usage patterns
- **Solution**: Tune GC settings, reduce object allocation

#### Inefficient algorithms
- **Symptoms**: CPU spikes during specific operations
- **Solution**: Optimize hot paths, add caching

#### Thread contention
- **Symptoms**: Lock contention, high thread count
- **Solution**: Reduce lock scope, use concurrent data structures

#### External dependencies
- **Symptoms**: CPU usage correlates with external calls
- **Solution**: Add caching, circuit breakers

## Resolution Steps

### Short-term
1. Scale horizontally if possible
2. Enable request throttling
3. Disable non-critical features

### Long-term
1. Profile and optimize hot paths
2. Implement caching strategies
3. Review and optimize algorithms
4. Consider async processing for heavy operations

## Prevention
- Set up CPU usage alerts at 70% threshold
- Regular performance profiling
- Load testing before releases
- Capacity planning reviews

## Related Incidents
- INC-2024-001: API service CPU spike due to regex backtracking
- INC-2024-015: GC pause causing CPU spikes in payment service
