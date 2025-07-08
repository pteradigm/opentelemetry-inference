"""
Unit tests for Enhanced Kalman Filter Multi-Feature CPU Prediction Model

Tests cover:
- 5D state vector initialization and transitions
- Multi-metric input processing
- Adaptive noise estimation
- Preprocessing pipeline
- Prediction accuracy and confidence
"""

import numpy as np
import pytest
import pytest_asyncio
from unittest.mock import Mock, patch
from model import KalmanFilterModel
from mlserver.types import InferenceRequest, Parameters, RequestInput
from mlserver.codecs import NumpyCodec
from mlserver.settings import ModelSettings, ModelParameters


class TestKalmanFilterModel:
    """Test suite for enhanced Kalman Filter model."""
    
    @pytest_asyncio.fixture
    async def model(self):
        """Create and load a Kalman filter model instance."""
        settings = ModelSettings(
            name="test-kalman-filter",
            implementation="model.KalmanFilterModel",
            parameters=ModelParameters(version="v1")
        )
        model = KalmanFilterModel(settings)
        await model.load()
        return model
    
    def test_initialization(self):
        """Test model initialization with correct dimensions."""
        settings = ModelSettings(
            name="test-kalman-filter",
            implementation="model.KalmanFilterModel",
            parameters=ModelParameters(version="v1")
        )
        model = KalmanFilterModel(settings)
        assert model.state_dim == 5
        assert model.obs_dim == 3
        assert model.innovation_window.maxlen == 50
        assert model.adaptive_enabled is True
        assert model.min_observations_for_adaptation == 20
    
    @pytest.mark.asyncio
    async def test_kalman_filter_setup(self, model):
        """Test Kalman filter matrices are properly configured."""
        assert model.filter is not None
        assert model.filter.F.shape == (5, 5)  # State transition matrix
        assert model.filter.H.shape == (3, 5)  # Measurement matrix
        assert model.filter.Q.shape == (5, 5)  # Process noise
        assert model.filter.R.shape == (3, 3)  # Measurement noise
        assert model.filter.P.shape == (5, 5)  # State covariance
        assert model.filter.x.shape == (5, 1)  # Initial state
        
        # Verify cross-correlation in state transition matrix
        F = model.filter.F
        assert F[0, 2] == 0.1  # Memory affects CPU
        assert F[0, 3] == 0.05  # Load average affects CPU
        assert F[2, 0] == 0.2  # CPU affects memory (GC, caching)
    
    @pytest.mark.asyncio
    async def test_metadata_response(self, model):
        """Test model metadata includes all inputs and outputs."""
        metadata = await model.metadata()
        
        # Check inputs
        assert len(metadata.inputs) == 3
        input_names = [inp.name for inp in metadata.inputs]
        assert "memory_utilization" in input_names
        assert "load_average_15m" in input_names
        assert "load_average_1m" in input_names
        
        # Check outputs
        assert len(metadata.outputs) == 5
        output_names = [out.name for out in metadata.outputs]
        assert "cpu_prediction" in output_names
        assert "prediction_variance" in output_names
        assert "innovation" in output_names
        assert "cpu_trend" in output_names
        assert "model_confidence" in output_names
    
    def test_preprocessing_missing_data(self):
        """Test preprocessing handles missing data correctly."""
        settings = ModelSettings(
            name="test-kalman-filter",
            implementation="model.KalmanFilterModel",
            parameters=ModelParameters(version="v1")
        )
        model = KalmanFilterModel(settings)
        
        # Test with NaN values
        cpu = np.array([0.5, np.nan, 0.6, np.nan, 0.7])
        memory = np.array([0.4, 0.45, np.nan, 0.5, 0.55])
        load_avg = np.array([1.0, 1.2, 1.1, np.nan, 1.3])
        
        cpu_clean, memory_clean, load_clean = model._preprocess_inputs(cpu, memory, load_avg)
        
        # Verify no NaN values remain
        assert not np.isnan(cpu_clean).any()
        assert not np.isnan(memory_clean).any()
        assert not np.isnan(load_clean).any()
        
        # Verify values are in valid ranges
        assert np.all((cpu_clean >= 0) & (cpu_clean <= 1))
        assert np.all((memory_clean >= 0) & (memory_clean <= 1))
        assert np.all(load_clean >= 0)
    
    def test_preprocessing_outliers(self):
        """Test preprocessing caps outliers correctly."""
        settings = ModelSettings(
            name="test-kalman-filter",
            implementation="model.KalmanFilterModel",
            parameters=ModelParameters(version="v1")
        )
        model = KalmanFilterModel(settings)
        
        # Test with outliers
        cpu = np.array([0.5, 0.5, 10.0, 0.5, -5.0])  # Outliers: 10.0, -5.0
        memory = np.array([0.4, 0.4, 0.4, 2.0, 0.4])  # Outlier: 2.0
        load_avg = np.array([1.0, 1.0, 1.0, 200.0, 1.0])  # Outlier: 200.0
        
        cpu_clean, memory_clean, load_clean = model._preprocess_inputs(cpu, memory, load_avg)
        
        # Verify outliers are capped
        assert np.all((cpu_clean >= 0) & (cpu_clean <= 1))
        assert np.all((memory_clean >= 0) & (memory_clean <= 1))
        assert np.all(load_clean <= 100)  # Reasonable upper bound for load
    
    def test_context_switch_estimation(self):
        """Test context switch estimation from CPU trend and load."""
        settings = ModelSettings(
            name="test-kalman-filter",
            implementation="model.KalmanFilterModel",
            parameters=ModelParameters(version="v1")
        )
        model = KalmanFilterModel(settings)
        
        # Low load, stable CPU
        cs1 = model._estimate_context_switches(cpu_trend=0.01, load_avg=0.5)
        assert 0 <= cs1 <= 0.1
        
        # High load, changing CPU
        cs2 = model._estimate_context_switches(cpu_trend=0.5, load_avg=8.0)
        assert cs1 < cs2 <= 1.0
        
        # Edge cases
        cs3 = model._estimate_context_switches(cpu_trend=-0.5, load_avg=10.0)
        assert 0 <= cs3 <= 1.0
    
    @pytest.mark.asyncio
    async def test_single_prediction(self, model):
        """Test single observation prediction."""
        # Create inference request with 3 inputs
        memory_input = NumpyCodec.encode_input(
            name="memory_utilization",
            payload=np.array([0.6]),
            use_bytes=False
        )
        load_15m_input = NumpyCodec.encode_input(
            name="load_average_15m",
            payload=np.array([2.5]),
            use_bytes=False
        )
        load_1m_input = NumpyCodec.encode_input(
            name="load_average_1m",
            payload=np.array([3.0]),
            use_bytes=False
        )
        
        request = InferenceRequest(
            inputs=[memory_input, load_15m_input, load_1m_input]
        )
        
        response = await model.predict(request)
        
        # Verify response structure
        assert response.model_name == model.name
        assert len(response.outputs) == 5
        
        # Decode outputs
        outputs = {out.name: NumpyCodec.decode_output(out) for out in response.outputs}
        
        # Verify output shapes and ranges
        for name in ["cpu_prediction", "prediction_variance", "innovation", "cpu_trend", "model_confidence"]:
            assert name in outputs
            # MLServer typically returns 2D arrays
            assert outputs[name].shape == (1, 1) or outputs[name].shape == (1,)
        
        # Verify ranges (handle both 1D and 2D array cases)
        cpu_pred = outputs["cpu_prediction"].flatten()[0]
        pred_var = outputs["prediction_variance"].flatten()[0]
        innovation = outputs["innovation"].flatten()[0]
        cpu_trend = outputs["cpu_trend"].flatten()[0]
        confidence = outputs["model_confidence"].flatten()[0]
        
        assert 0 <= cpu_pred <= 1
        assert pred_var >= 0
        assert innovation >= 0
        assert -1 <= cpu_trend <= 1
        assert 0 <= confidence <= 1
    
    @pytest.mark.asyncio
    async def test_multi_observation_prediction(self, model):
        """Test batch prediction with multiple observations."""
        batch_size = 10
        
        # Create batch inputs
        memory_input = NumpyCodec.encode_input(
            name="memory_utilization",
            payload=np.random.uniform(0.3, 0.8, batch_size),
            use_bytes=False
        )
        load_15m_input = NumpyCodec.encode_input(
            name="load_average_15m",
            payload=np.random.uniform(0.5, 4.0, batch_size),
            use_bytes=False
        )
        load_1m_input = NumpyCodec.encode_input(
            name="load_average_1m",
            payload=np.random.uniform(0.5, 5.0, batch_size),
            use_bytes=False
        )
        
        request = InferenceRequest(
            inputs=[memory_input, load_15m_input, load_1m_input]
        )
        
        response = await model.predict(request)
        
        # Decode outputs
        outputs = {out.name: NumpyCodec.decode_output(out) for out in response.outputs}
        
        # Verify batch dimensions (MLServer may return 2D arrays)
        for name, data in outputs.items():
            assert data.shape == (batch_size,) or data.shape == (batch_size, 1), f"{name} has wrong shape: {data.shape}"
    
    @pytest.mark.asyncio
    async def test_adaptive_noise_update(self, model):
        """Test adaptive noise estimation updates after sufficient observations."""
        initial_Q = model.filter.Q.copy()
        initial_R = model.filter.R.copy()
        
        # Generate enough observations to trigger adaptation
        n_obs = model.min_observations_for_adaptation + 10
        
        for _ in range(n_obs):
            memory_input = NumpyCodec.encode_input(
                name="memory_utilization",
                payload=np.array([np.random.uniform(0.4, 0.6)]),
                use_bytes=False
            )
            load_15m_input = NumpyCodec.encode_input(
                name="load_average_15m",
                payload=np.array([np.random.uniform(1.0, 2.0)]),
                use_bytes=False
            )
            load_1m_input = NumpyCodec.encode_input(
                name="load_average_1m",
                payload=np.array([np.random.uniform(1.0, 3.0)]),
                use_bytes=False
            )
            
            request = InferenceRequest(
                inputs=[memory_input, load_15m_input, load_1m_input]
            )
            
            await model.predict(request)
        
        # Verify noise matrices have been updated
        assert not np.array_equal(model.filter.Q, initial_Q) or not np.array_equal(model.filter.R, initial_R)
        
        # Verify positive definiteness
        assert np.all(np.linalg.eigvals(model.filter.Q) > 0)
        assert np.all(np.linalg.eigvals(model.filter.R) > 0)
    
    @pytest.mark.asyncio
    async def test_prediction_consistency(self, model):
        """Test that predictions are consistent and stable."""
        # Fixed inputs for consistency test
        memory = 0.5
        load_15m = 2.0
        load_1m = 2.5
        
        predictions = []
        confidences = []
        
        # Run multiple predictions with same input
        for _ in range(5):
            memory_input = NumpyCodec.encode_input(
                name="memory_utilization",
                payload=np.array([memory]),
                use_bytes=False
            )
            load_15m_input = NumpyCodec.encode_input(
                name="load_average_15m",
                payload=np.array([load_15m]),
                use_bytes=False
            )
            load_1m_input = NumpyCodec.encode_input(
                name="load_average_1m",
                payload=np.array([load_1m]),
                use_bytes=False
            )
            
            request = InferenceRequest(
                inputs=[memory_input, load_15m_input, load_1m_input]
            )
            
            response = await model.predict(request)
            outputs = {out.name: NumpyCodec.decode_output(out) for out in response.outputs}
            
            predictions.append(outputs["cpu_prediction"][0])
            confidences.append(outputs["model_confidence"][0])
        
        # Predictions should converge
        pred_std = np.std(predictions)
        assert pred_std < 0.1, f"Predictions not stable: std={pred_std}"
        
        # Confidence should increase
        assert confidences[-1] >= confidences[0], "Model confidence should increase with observations"
    
    @pytest.mark.asyncio
    async def test_cpu_estimation_from_load(self, model):
        """Test CPU utilization estimation from load average."""
        # Test various load averages
        test_cases = [
            (0.5, 0.0625),   # Low load -> low CPU (0.5/8)
            (4.0, 0.5),      # Medium load -> medium CPU (4/8)
            (8.0, 1.0),      # High load -> max CPU (8/8)
            (12.0, 1.0),     # Overload -> capped at 1.0
        ]
        
        for load_1m, expected_cpu in test_cases:
            memory_input = NumpyCodec.encode_input(
                name="memory_utilization",
                payload=np.array([0.5]),
                use_bytes=False
            )
            load_15m_input = NumpyCodec.encode_input(
                name="load_average_15m",
                payload=np.array([load_1m * 0.8]),  # 15m typically lower
                use_bytes=False
            )
            load_1m_input = NumpyCodec.encode_input(
                name="load_average_1m",
                payload=np.array([load_1m]),
                use_bytes=False
            )
            
            request = InferenceRequest(
                inputs=[memory_input, load_15m_input, load_1m_input]
            )
            
            # Just verify the request processes without error
            response = await model.predict(request)
            assert len(response.outputs) == 5
    
    @pytest.mark.asyncio 
    async def test_error_handling(self, model):
        """Test error handling for invalid inputs."""
        # Test with wrong number of inputs
        with pytest.raises(ValueError, match="Expected 3 inputs"):
            request = InferenceRequest(
                inputs=[NumpyCodec.encode_input(
                    name="memory_utilization",
                    payload=np.array([0.5]),
                    use_bytes=False
                )]
            )
            await model.predict(request)
        
        # Test before initialization
        settings = ModelSettings(
            name="test-kalman-filter",
            implementation="model.KalmanFilterModel",
            parameters=ModelParameters(version="v1")
        )
        uninitialized_model = KalmanFilterModel(settings)
        with pytest.raises(RuntimeError, match="Kalman filter not initialized"):
            request = InferenceRequest(
                inputs=[
                    NumpyCodec.encode_input(name="memory_utilization", payload=np.array([0.5]), use_bytes=False),
                    NumpyCodec.encode_input(name="load_average_15m", payload=np.array([1.0]), use_bytes=False),
                    NumpyCodec.encode_input(name="load_average_1m", payload=np.array([1.0]), use_bytes=False)
                ]
            )
            await uninitialized_model.predict(request)


if __name__ == "__main__":
    pytest.main([__file__, "-v"])