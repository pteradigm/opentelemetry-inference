# TASK-0008-00-00: Dynamic Model Metadata Discovery

## Task Header
- **Task ID**: TASK-0008-00-00
- **Status**: COMPLETED
- **Created**: 2025-06-29
- **Modified**: 2025-06-29
- **Completed**: 2025-06-29
- **Assignee**: @rbellamy
- **Priority**: P1 (High)
- **Dependencies**: None
- **Estimated Effort**: M (Medium - 2-3 days)
- **Labels**: enhancement, processor, grpc, kserve

## User Story
**As a** developer using the OpenTelemetry Inference Processor  
**I want** the processor to automatically discover model output metadata at startup  
**So that** I don't need to manually configure output names and types in the YAML configuration

## Context & Research

### Current State
- The processor requires manual configuration of output names and types in YAML rules
- The KServe v2 ModelMetadata RPC is available but not utilized
- This approach doesn't scale well for models with many outputs or dynamic output names

### API Documentation Review
The KServe v2 protocol provides:
- `ModelMetadata` RPC for querying model input/output specifications
- `TensorMetadata` structure with name, datatype, and shape information
- Available at processor startup time via gRPC

### Technical Research
- The processor already has the proto definitions for ModelMetadata
- The gRPC client is established during Start()
- Metadata can be cached per model to avoid repeated queries

## Acceptance Criteria

### Functional Requirements
1. **FR1**: Processor queries ModelMetadata for each unique model in rules during Start()
2. **FR2**: Output names and types are automatically discovered if not explicitly configured
3. **FR3**: Explicit configuration in YAML takes precedence over discovered metadata
4. **FR4**: Processor logs discovered metadata at INFO level for debugging
5. **FR5**: Processor gracefully handles models that don't support metadata queries

### Non-Functional Requirements
1. **NFR1**: Metadata queries add <100ms to processor startup time
2. **NFR2**: Metadata is cached to avoid repeated queries
3. **NFR3**: No breaking changes to existing configurations
4. **NFR4**: Clear error messages when metadata discovery fails

## Behavioral Specifications

### Scenario 1: Automatic Output Discovery
```gherkin
Given a model "simple-scaler" with outputs ["prediction", "confidence"]
And a rule configuration with no output specifications
When the processor starts
Then it queries ModelMetadata for "simple-scaler"
And automatically creates output metrics "prediction" and "confidence"
```

### Scenario 2: Explicit Configuration Override
```gherkin
Given a model "simple-scaler" with output "prediction"
And a rule configuration specifying output name "system.cpu.predicted"
When the processor starts
Then it uses the configured name "system.cpu.predicted"
And does not use the discovered name "prediction"
```

### Scenario 3: Metadata Query Failure
```gherkin
Given a model that doesn't support ModelMetadata RPC
When the processor starts
Then it logs a warning about metadata unavailability
And falls back to requiring explicit output configuration
```

## Implementation Plan

### Phase 1: Setup (Day 1)
- [ ] Create model metadata cache structure
- [ ] Add metadata query logic to processor Start()
- [ ] Update internal rule structure to support discovered metadata

### Phase 2: Development (Day 2)
- [ ] Implement ModelMetadata RPC calls for each unique model
- [ ] Create fallback logic for missing metadata
- [ ] Update rule validation to make outputs optional when metadata available
- [ ] Implement configuration merge logic (explicit config wins)

### Phase 3: Validation (Day 3)
- [ ] Write unit tests with mock metadata responses
- [ ] Create integration tests with MLServer
- [ ] Test backward compatibility with existing configs
- [ ] Performance testing for startup time impact

### Phase 4: Documentation
- [ ] Update README with metadata discovery feature
- [ ] Add examples showing automatic discovery
- [ ] Document configuration precedence rules

## Test Plan

### Unit Tests
1. Test metadata query and caching logic
2. Test configuration merge (explicit vs discovered)
3. Test error handling for failed metadata queries
4. Test output name generation from tensor names

### Integration Tests
1. Test with MLServer models providing metadata
2. Test with models not supporting metadata
3. Test processor restart with cached metadata
4. Test multiple models with different metadata

### E2E Tests
1. Deploy full pipeline with metadata discovery
2. Verify metrics are created with correct names
3. Test configuration updates without restart

## Definition of Done
- [x] All unit tests passing
- [x] All integration tests passing  
- [x] Documentation updated
- [x] Code reviewed and approved
- [x] Performance impact measured and acceptable
- [x] Backward compatibility verified
- [x] CLAUDE.md updated with new feature

## Completion Summary

Successfully implemented dynamic model metadata discovery using the KServe v2 ModelMetadata RPC. Key achievements:

### Implementation
- Added `modelMetadata` cache to processor for storing discovered metadata
- Implemented `queryModelMetadata()` method that queries model metadata during processor startup
- Created `mergeDiscoveredOutputs()` method to merge discovered metadata with configured outputs
- Added `convertKServeDataType()` utility to map KServe data types to internal types
- Updated configuration validation to make outputs optional when metadata is available

### Testing
- Created comprehensive unit tests covering metadata discovery scenarios
- Added mock server support for metadata responses
- Tested backward compatibility with existing configurations
- Verified error handling when metadata is not available

### Configuration Changes
- Outputs are now optional in YAML configuration
- Processor automatically discovers output names and types from model metadata
- Explicit configuration still takes precedence over discovered metadata
- Updated demo and example configurations to demonstrate the feature

### Documentation
- Updated CLAUDE.md with metadata discovery feature description
- Created example configuration showing automatic discovery
- Updated configuration structure documentation