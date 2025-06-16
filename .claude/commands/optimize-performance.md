Please analyze and optimize the performance of: $ARGUMENTS

## Instructions

1. **Performance analysis**:
   - Profile the code to identify bottlenecks
   - Measure current performance with benchmarks
   - Analyze memory usage patterns
   - Identify expensive operations (I/O, computation, memory allocation)

2. **Bottleneck identification**:
   - Find the slowest functions or operations
   - Identify unnecessary work or redundant operations
   - Look for inefficient algorithms or data structures
   - Check for memory leaks or excessive allocations

3. **Optimization strategy**:
   - Prioritize optimizations by impact vs effort
   - Focus on the biggest performance gains first
   - Consider algorithmic improvements before micro-optimizations
   - Plan optimizations that don't sacrifice readability

4. **Algorithm optimization**:
   - Replace inefficient algorithms with better ones
   - Optimize data structures for the use case
   - Reduce algorithm complexity (O(n²) → O(n log n))
   - Eliminate unnecessary loops or iterations

5. **Memory optimization**:
   - Reduce memory allocations in hot paths
   - Reuse objects and buffers where possible
   - Fix memory leaks and excessive retention
   - Optimize data structure memory layout

6. **I/O optimization**:
   - Batch operations to reduce I/O calls
   - Use appropriate buffer sizes
   - Implement caching for frequently accessed data
   - Consider async/parallel operations

7. **Implementation and verification**:
   - Implement optimizations incrementally
   - Benchmark before and after each change
   - Ensure functionality remains correct
   - Profile the optimized code to verify improvements

## Performance Optimization Techniques

### Algorithmic Improvements
- [ ] Replace O(n²) algorithms with O(n log n) or better
- [ ] Use appropriate data structures (maps vs slices, trees vs arrays)
- [ ] Implement caching for expensive computations
- [ ] Eliminate redundant calculations

### Memory Optimization
- [ ] Reduce memory allocations in loops
- [ ] Pool and reuse expensive objects
- [ ] Use streaming instead of loading everything into memory
- [ ] Optimize data structure size and alignment

### Concurrency Optimization
- [ ] Parallelize independent operations
- [ ] Use worker pools for CPU-intensive tasks
- [ ] Implement async I/O where beneficial
- [ ] Optimize lock contention and critical sections

### I/O Optimization
- [ ] Batch database queries and updates
- [ ] Use appropriate buffer sizes for file operations
- [ ] Implement connection pooling
- [ ] Cache frequently accessed data

### Language-Specific Optimizations
- [ ] Use efficient string operations (builders vs concatenation)
- [ ] Optimize slice operations and growth
- [ ] Use appropriate numeric types
- [ ] Leverage compiler optimizations

## Benchmarking and Profiling

### Before Optimization
```go
func BenchmarkOriginal(b *testing.B) {
    for i := 0; i < b.N; i++ {
        // Original implementation
    }
}
```

### After Optimization
```go
func BenchmarkOptimized(b *testing.B) {
    for i := 0; i < b.N; i++ {
        // Optimized implementation
    }
}
```

### Memory Profiling
- Use tools like `go tool pprof` for Go
- Monitor heap usage and garbage collection
- Track allocation patterns and frequencies

### CPU Profiling
- Identify hot functions and call paths
- Measure time spent in different operations
- Look for unexpected expensive operations

## Optimization Checklist

### Analysis
- [ ] Current performance baseline established
- [ ] Bottlenecks identified and prioritized
- [ ] Profiling data collected and analyzed
- [ ] Optimization targets defined

### Implementation
- [ ] Algorithmic improvements implemented
- [ ] Memory usage optimized
- [ ] I/O operations optimized
- [ ] Concurrency utilized appropriately

### Verification
- [ ] Performance improvements measured
- [ ] Functionality still correct
- [ ] No new bugs introduced
- [ ] Resource usage improved

### Documentation
- [ ] Optimization rationale documented
- [ ] Performance benchmarks recorded
- [ ] Trade-offs and limitations noted
- [ ] Maintenance considerations documented

## Success Confirmation

After optimization, confirm:
- 📈 Significant performance improvement achieved
- 📊 Benchmarks show measurable gains
- ✅ All functionality preserved and tests pass
- 💾 Memory usage optimized appropriately
- 📝 Changes documented with performance metrics