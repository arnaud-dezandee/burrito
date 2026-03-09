import axios from 'axios';

import { Stacks } from '@/clients/stacks/types.ts';

export const fetchStacks = async () => {
  const response = await axios.get<Stacks>(
    `${import.meta.env.VITE_API_BASE_URL}/stacks`
  );
  return response.data;
};

export const syncStack = async (namespace: string, name: string) => {
  return axios.post(
    `${import.meta.env.VITE_API_BASE_URL}/stacks/${namespace}/${name}/sync`
  );
};

export const applyStack = async (namespace: string, name: string) => {
  return axios.post(
    `${import.meta.env.VITE_API_BASE_URL}/stacks/${namespace}/${name}/apply`
  );
};
