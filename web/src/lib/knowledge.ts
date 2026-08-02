import { knowledgeServiceClient } from "@/connect";
import {
  CreateKnowledgeBaseRequestSchema,
  DeleteDocumentRequestSchema,
  DeleteKnowledgeBaseRequestSchema,
  type Document,
  type KnowledgeBase,
  KnowledgeBaseSchema,
  ListDocumentsRequestSchema,
  ListKnowledgeBasesRequestSchema,
  UpdateKnowledgeBaseRequestSchema,
  UploadDocumentRequestSchema,
} from "@/types/proto/api/v1/knowledge_service_pb";
import { create } from "@bufbuild/protobuf";
import { FieldMaskSchema } from "@bufbuild/protobuf/wkt";

// Re-export for convenience
export type { KnowledgeBase, Document };

/** Fetch all knowledge bases */
export async function listKnowledgeBases(): Promise<KnowledgeBase[]> {
  const request = create(ListKnowledgeBasesRequestSchema, { pageSize: 100 });
  const response = await knowledgeServiceClient.listKnowledgeBases(request);
  return response.knowledgeBases;
}

/** Create a new knowledge base */
export async function createKnowledgeBase(title: string, description: string): Promise<KnowledgeBase> {
  const request = create(CreateKnowledgeBaseRequestSchema, {
    knowledgeBase: create(KnowledgeBaseSchema, { title, description }),
  });
  return await knowledgeServiceClient.createKnowledgeBase(request);
}

/** Update a knowledge base */
export async function updateKnowledgeBase(
  name: string,
  title: string,
  description: string,
): Promise<KnowledgeBase> {
  const request = create(UpdateKnowledgeBaseRequestSchema, {
    knowledgeBase: create(KnowledgeBaseSchema, { name, title, description }),
    updateMask: create(FieldMaskSchema, { paths: ["title", "description"] }),
  });
  return await knowledgeServiceClient.updateKnowledgeBase(request);
}

/** Delete a knowledge base by name */
export async function deleteKnowledgeBase(name: string): Promise<void> {
  const request = create(DeleteKnowledgeBaseRequestSchema, { name });
  await knowledgeServiceClient.deleteKnowledgeBase(request);
}

/** List documents in a knowledge base */
export async function listDocuments(parent: string): Promise<Document[]> {
  const request = create(ListDocumentsRequestSchema, { parent, pageSize: 100 });
  const response = await knowledgeServiceClient.listDocuments(request);
  return response.documents;
}

/** Upload a document to a knowledge base */
export async function uploadDocument(parent: string, filename: string, content: Uint8Array): Promise<Document> {
  const request = create(UploadDocumentRequestSchema, { parent, filename, content });
  return await knowledgeServiceClient.uploadDocument(request);
}

/** Delete a document */
export async function deleteDocument(name: string): Promise<void> {
  const request = create(DeleteDocumentRequestSchema, { name });
  await knowledgeServiceClient.deleteDocument(request);
}
