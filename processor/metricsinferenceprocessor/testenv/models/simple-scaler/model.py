"""
Simple scaling model for KServe testing.
This model multiplies input metrics by a scaling factor.
"""

from mlserver import MLModel
from mlserver.types import InferenceRequest, InferenceResponse, MetadataModelResponse, MetadataTensor
from mlserver.codecs import NumpyCodec


class SimpleScalerModel(MLModel):
    """
    A simple model that scales input metrics by a configurable factor.
    This demonstrates basic KServe v2 protocol integration for single-input operations.
    """
    
    async def load(self) -> bool:
        """Load the model and set scaling factor."""
        # Get scaling factor from model settings, default to 2.0
        scale_factor_param = getattr(self.settings.parameters, "scale_factor", 2.0)
        self.scale_factor = float(scale_factor_param)
        
        self.ready = True
        return self.ready
    
    async def metadata(self) -> MetadataModelResponse:
        """
        Provide model metadata including input and output specifications.
        
        Returns:
            MetadataModelResponse containing model metadata
        """
        # Define input metadata - flexible for different metric names
        inputs = [
            MetadataTensor(
                name="input_tensor",
                datatype="FP64", 
                shape=[-1],  # Variable size input
            )
        ]
        
        # Define output metadata for scale operation
        outputs = [
            MetadataTensor(
                name="scaled_result", 
                datatype="FP64",
                shape=[-1],
            )
        ]
        
        return MetadataModelResponse(
            name=self.name,
            platform="python",
            versions=[self.version] if self.version else ["v1"],
            inputs=inputs,
            outputs=outputs
        )
    
    async def predict(self, payload: InferenceRequest) -> InferenceResponse:
        """
        Perform scaling inference on the input metrics.
        
        Args:
            payload: InferenceRequest containing input tensors
            
        Returns:
            InferenceResponse with scaled output tensors
        """
        # Get model metadata to validate inputs and determine outputs
        model_metadata = await self.metadata()
        expected_inputs = model_metadata.inputs
        expected_outputs = model_metadata.outputs
        
        # Validate input count matches metadata (expecting exactly 1 input)
        if len(payload.inputs) != len(expected_inputs):
            raise ValueError(f"Expected {len(expected_inputs)} inputs, got {len(payload.inputs)}")
        
        outputs = []
        
        # Process each input according to metadata (should be only 1 for scaler)
        for i, request_input in enumerate(payload.inputs):
            expected_input = expected_inputs[i]
            
            # Validate input data type matches expected
            # Convert enum to string for comparison
            request_datatype = request_input.datatype
            if hasattr(request_datatype, 'value'):
                request_datatype = request_datatype.value
            if str(request_datatype) != str(expected_input.datatype):
                raise ValueError(f"Input {i} datatype mismatch: expected {expected_input.datatype}, got {request_datatype}")
            
            # Decode input tensor from the request
            input_data = NumpyCodec.decode_input(request_input)
            
            # Scale the input data
            scaled_data = input_data * self.scale_factor
            
            # Create outputs according to metadata specifications
            for expected_output in expected_outputs:
                response_output = NumpyCodec.encode_output(
                    name=expected_output.name,
                    payload=scaled_data,
                    use_bytes=False
                )
                outputs.append(response_output)
        
        return InferenceResponse(
            model_name=self.name,
            model_version=self.version,
            outputs=outputs
        )