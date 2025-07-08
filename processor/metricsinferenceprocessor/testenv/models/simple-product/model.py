"""
Simple product model for KServe testing.
This model multiplies multiple input metrics together.
Useful for calculating memory used = utilization % * total limit.
"""

from mlserver import MLModel
from mlserver.types import InferenceRequest, InferenceResponse, MetadataModelResponse, MetadataTensor
from mlserver.codecs import NumpyCodec


class SimpleProductModel(MLModel):
    """
    A simple model that multiplies multiple input metrics together.
    This demonstrates basic KServe v2 protocol integration for product operations.
    Specifically designed for memory calculations: utilization * limit = used.
    """
    
    async def load(self) -> bool:
        """Load the model."""
        self.ready = True
        return self.ready
    
    async def metadata(self) -> MetadataModelResponse:
        """
        Provide model metadata including input and output specifications.
        
        Returns:
            MetadataModelResponse containing model metadata
        """
        # Define input metadata - two inputs for product operation
        inputs = [
            MetadataTensor(
                name="input_tensor_1",
                datatype="FP64", 
                shape=[-1],  # Variable size input
            ),
            MetadataTensor(
                name="input_tensor_2",
                datatype="FP64", 
                shape=[-1],  # Variable size input
            )
        ]
        
        # Define output metadata for product operation
        outputs = [
            MetadataTensor(
                name="product_result",
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
        Perform product inference on the input metrics.
        
        Args:
            payload: InferenceRequest containing input tensors
            
        Returns:
            InferenceResponse with multiplied output tensor
        """
        # Get model metadata to validate inputs and determine outputs
        model_metadata = await self.metadata()
        expected_inputs = model_metadata.inputs
        expected_outputs = model_metadata.outputs
        
        # Validate input count matches metadata
        if len(payload.inputs) != len(expected_inputs):
            raise ValueError(f"Expected {len(expected_inputs)} inputs, got {len(payload.inputs)}")
        
        # Validate and decode inputs according to metadata
        decoded_inputs = []
        for i, request_input in enumerate(payload.inputs):
            expected_input = expected_inputs[i]
            
            # Validate input data type matches expected
            # Convert enum to string for comparison
            request_datatype = request_input.datatype
            if hasattr(request_datatype, 'value'):
                request_datatype = request_datatype.value
            if str(request_datatype) != str(expected_input.datatype):
                raise ValueError(f"Input {i} datatype mismatch: expected {expected_input.datatype}, got {request_datatype}")
            
            # Decode the input tensor
            input_data = NumpyCodec.decode_input(request_input)
            decoded_inputs.append(input_data)
        
        # Perform the product operation
        if len(decoded_inputs) >= 2:
            # Multiply all inputs element-wise
            result = decoded_inputs[0]
            for input_data in decoded_inputs[1:]:
                result = result * input_data
        elif len(decoded_inputs) == 1:
            # Single input case - pass through (identity operation)
            result = decoded_inputs[0]
        else:
            raise ValueError("No inputs provided for product operation")
        
        # Create outputs according to metadata specifications
        outputs = []
        for expected_output in expected_outputs:
            response_output = NumpyCodec.encode_output(
                name=expected_output.name,
                payload=result,
                use_bytes=False
            )
            outputs.append(response_output)
        
        return InferenceResponse(
            model_name=self.name,
            model_version=self.version,
            outputs=outputs
        )