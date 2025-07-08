import axios from 'axios';

export const fetchPlan = async (
  namespace: string,
  layer: string,
  runId: string,
  attemptId: number | null
) => {
  const response = await axios.get<string>(
    `${import.meta.env.VITE_API_BASE_URL}/plan/${namespace}/${layer}/${runId}/${attemptId}`,
    {
      responseType: 'text',
    }
  );
  return response.data;
};