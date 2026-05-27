import { FormEvent, useState, useEffect } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  useSkill,
  useCreateSkill,
  useUpdateSkill,
  useImportSkill,
} from "../hooks/useSkills";
import type { SkillFileInput } from "../hooks/useSkills";
import { useToast } from "../components/Toast";

/* ─── Validation ─── */

const SKILL_NAME_REGEX = /^[a-z0-9][a-z0-9-]*$/;

function validateSkillName(name: string): string | null {
  if (!name) return "Name is required";
  if (name.length > 64) return "Name must not exceed 64 characters";
  if (!SKILL_NAME_REGEX.test(name))
    return "Name must match pattern: lowercase alphanumeric with hyphens, starting with a letter or number";
  return null;
}

function validateDescription(desc: string): string | null {
  if (desc.length > 255) return "Description must not exceed 255 characters";
  return null;
}

function validateContent(content: string): string | null {
  if (!content.trim()) return "Content is required";
  if (new Blob([content]).size > 204800)
    return "Content must be 200KB or smaller";
  return null;
}

function validateFilePath(path: string): string | null {
  if (!path.trim()) return "File path is required";
  if (path.length > 512) return "Path must not exceed 512 characters";
  return null;
}

function validateFileContent(content: string): string | null {
  if (new Blob([content]).size > 1048576)
    return "File content must be 1MB or smaller";
  return null;
}

function validateImportUrl(url: string): string | null {
  if (!url.trim()) return "URL is required";
  try {
    new URL(url);
  } catch {
    return "Must be a valid URL";
  }
  return null;
}

/* ─── Types ─── */

interface FileEntry {
  path: string;
  content: string;
}

/* ─── Component ─── */

export default function SkillForm() {
  const navigate = useNavigate();
  const { showToast } = useToast();
  const { id } = useParams<{ id: string }>();
  const isEditMode = !!id;

  // Fetch existing skill for edit mode
  const { data: existingSkill, isLoading: skillLoading } = useSkill(id ?? "");

  // Form state
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [content, setContent] = useState("");
  const [files, setFiles] = useState<FileEntry[]>([]);

  // Import state
  const [importUrl, setImportUrl] = useState("");
  const [showImport, setShowImport] = useState(false);

  // Validation state
  const [fieldErrors, setFieldErrors] = useState<Record<string, string | null>>(
    {}
  );
  const [submitError, setSubmitError] = useState<string | null>(null);

  // Mutations
  const createSkill = useCreateSkill();
  const updateSkill = useUpdateSkill(id ?? "");
  const importSkill = useImportSkill();

  // Pre-populate fields in edit mode
  useEffect(() => {
    if (isEditMode && existingSkill) {
      setName(existingSkill.name);
      setDescription(existingSkill.description);
      setContent(existingSkill.content);
      setFiles(
        existingSkill.files.map((f) => ({
          path: f.path,
          content: f.content ?? "",
        }))
      );
    }
  }, [isEditMode, existingSkill]);

  /* ─── Field validation on blur ─── */

  function onBlurField(field: string) {
    let err: string | null = null;
    switch (field) {
      case "name":
        err = validateSkillName(name);
        break;
      case "description":
        err = validateDescription(description);
        break;
      case "content":
        err = validateContent(content);
        break;
    }
    setFieldErrors((prev) => ({ ...prev, [field]: err }));
  }

  /* ─── Files Management ─── */

  function addFile() {
    setFiles([...files, { path: "", content: "" }]);
  }

  function removeFile(index: number) {
    setFiles(files.filter((_, i) => i !== index));
    // Clear related errors
    setFieldErrors((prev) => {
      const next = { ...prev };
      delete next[`file_path_${index}`];
      delete next[`file_content_${index}`];
      return next;
    });
  }

  function updateFile(index: number, field: "path" | "content", value: string) {
    setFiles(
      files.map((f, i) => (i === index ? { ...f, [field]: value } : f))
    );
  }

  function validateFileField(index: number, field: "path" | "content") {
    const file = files[index];
    if (!file) return;
    const err =
      field === "path"
        ? validateFilePath(file.path)
        : validateFileContent(file.content);
    setFieldErrors((prev) => ({
      ...prev,
      [`file_${field}_${index}`]: err,
    }));
  }

  /* ─── Import from URL ─── */

  function handleImport() {
    const urlErr = validateImportUrl(importUrl);
    if (urlErr) {
      setFieldErrors((prev) => ({ ...prev, import_url: urlErr }));
      return;
    }
    setFieldErrors((prev) => ({ ...prev, import_url: null }));

    importSkill.mutate(
      { url: importUrl },
      {
        onSuccess: (skill) => {
          showToast("Skill imported successfully", "success");
          navigate(`/skills/${skill.id}/edit`);
        },
        onError: (err: Error) => {
          showToast(err.message || "Failed to import skill", "error");
        },
      }
    );
  }

  /* ─── Submit ─── */

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSubmitError(null);

    // Validate all fields
    const errors: Record<string, string | null> = {};
    errors.name = validateSkillName(name);
    errors.description = validateDescription(description);
    errors.content = validateContent(content);

    // Validate files
    files.forEach((file, i) => {
      errors[`file_path_${i}`] = validateFilePath(file.path);
      errors[`file_content_${i}`] = validateFileContent(file.content);
    });

    setFieldErrors(errors);

    // Check if any errors exist
    const hasErrors = Object.values(errors).some((e) => e !== null);
    if (hasErrors) return;

    // Build payload
    const fileInputs: SkillFileInput[] = files.map((f) => ({
      path: f.path,
      content: f.content,
    }));

    if (isEditMode) {
      updateSkill.mutate(
        {
          name,
          description,
          content,
          files: fileInputs,
        },
        {
          onSuccess: () => {
            showToast("Skill updated successfully", "success");
            navigate("/skills");
          },
          onError: (err: Error) => {
            setSubmitError(err.message || "Failed to update skill");
          },
        }
      );
    } else {
      createSkill.mutate(
        {
          name,
          description,
          content,
          files: fileInputs,
        },
        {
          onSuccess: () => {
            showToast("Skill created successfully", "success");
            navigate("/skills");
          },
          onError: (err: Error) => {
            setSubmitError(err.message || "Failed to create skill");
          },
        }
      );
    }
  }

  const isPending = createSkill.isPending || updateSkill.isPending;

  if (isEditMode && skillLoading) {
    return (
      <div className="max-w-3xl mx-auto px-6 py-8">
        <p className="text-sm text-gray-500">Loading skill...</p>
      </div>
    );
  }

  return (
    <div className="max-w-3xl mx-auto px-6 py-8">
      <div className="mb-6">
        <button
          type="button"
          onClick={() => navigate("/skills")}
          className="text-sm text-gray-500 hover:text-gray-700 mb-2 inline-flex items-center gap-1"
        >
          ← Back to Skills
        </button>
        <h1 className="text-2xl font-semibold text-gray-900">
          {isEditMode ? "Edit Skill" : "Create Skill"}
        </h1>
        <p className="mt-1 text-sm text-gray-500">
          {isEditMode
            ? "Update your skill's content, metadata, and supporting files."
            : "Define a new skill with instructions and optional supporting files."}
        </p>
      </div>

      {/* Import from URL section */}
      {!isEditMode && (
        <div className="mb-6 border border-gray-200 rounded-md p-4">
          <button
            type="button"
            onClick={() => setShowImport(!showImport)}
            className="text-sm font-medium text-blue-600 hover:text-blue-700 inline-flex items-center gap-1"
          >
            {showImport ? "▾" : "▸"} Import from URL
          </button>
          {showImport && (
            <div className="mt-3">
              <p className="text-xs text-gray-500 mb-2">
                Import a skill from a URL pointing to a SKILL.md file (supports
                GitHub URLs).
              </p>
              <div className="flex items-start gap-2">
                <div className="flex-1">
                  <input
                    type="text"
                    value={importUrl}
                    onChange={(e) => setImportUrl(e.target.value)}
                    onBlur={() => {
                      if (importUrl) {
                        const err = validateImportUrl(importUrl);
                        setFieldErrors((prev) => ({
                          ...prev,
                          import_url: err,
                        }));
                      }
                    }}
                    placeholder="https://github.com/user/repo/blob/main/.skills/my-skill/SKILL.md"
                    className="block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                  />
                  {fieldErrors.import_url && (
                    <p className="mt-1 text-xs text-red-600">
                      {fieldErrors.import_url}
                    </p>
                  )}
                </div>
                <button
                  type="button"
                  onClick={handleImport}
                  disabled={importSkill.isPending}
                  className="inline-flex items-center px-3 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50"
                >
                  {importSkill.isPending ? "Importing…" : "Import"}
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {submitError && (
        <div className="rounded-md bg-red-50 border border-red-200 p-3 text-sm text-red-700 mb-6">
          {submitError}
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-6">
        {/* Name */}
        <div>
          <label
            htmlFor="skill-name"
            className="block text-sm font-medium text-gray-700"
          >
            Name <span className="text-red-500">*</span>
          </label>
          <input
            id="skill-name"
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            onBlur={() => onBlurField("name")}
            maxLength={64}
            placeholder="my-skill-name"
            className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
          <p className="mt-1 text-xs text-gray-500">
            1–64 characters. Lowercase letters, numbers, and hyphens. Must start
            with a letter or number.
          </p>
          {fieldErrors.name && (
            <p className="mt-1 text-xs text-red-600">{fieldErrors.name}</p>
          )}
        </div>

        {/* Description */}
        <div>
          <label
            htmlFor="skill-description"
            className="block text-sm font-medium text-gray-700"
          >
            Description
          </label>
          <input
            id="skill-description"
            type="text"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            onBlur={() => onBlurField("description")}
            maxLength={255}
            placeholder="A brief description of what this skill provides"
            className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
          <p className="mt-1 text-xs text-gray-500">
            {description.length}/255 characters
          </p>
          {fieldErrors.description && (
            <p className="mt-1 text-xs text-red-600">
              {fieldErrors.description}
            </p>
          )}
        </div>

        {/* Content (Markdown Editor) */}
        <div>
          <label
            htmlFor="skill-content"
            className="block text-sm font-medium text-gray-700"
          >
            Content <span className="text-red-500">*</span>
          </label>
          <textarea
            id="skill-content"
            value={content}
            onChange={(e) => setContent(e.target.value)}
            onBlur={() => onBlurField("content")}
            placeholder="# Skill Instructions&#10;&#10;Write your skill content in markdown format..."
            className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 resize-y font-mono"
            style={{ minHeight: "400px" }}
          />
          <p className="mt-1 text-xs text-gray-500">
            Markdown content for the skill. This becomes the SKILL.md file.
          </p>
          {fieldErrors.content && (
            <p className="mt-1 text-xs text-red-600">{fieldErrors.content}</p>
          )}
        </div>

        {/* Supporting Files */}
        <div>
          <div className="flex items-center justify-between mb-2">
            <label className="block text-sm font-medium text-gray-700">
              Supporting Files
            </label>
            <button
              type="button"
              onClick={addFile}
              className="text-xs text-blue-600 hover:text-blue-700"
            >
              + Add File
            </button>
          </div>
          {files.length === 0 && (
            <p className="text-xs text-gray-500">
              No supporting files. Click "Add File" to include example code,
              templates, or schemas.
            </p>
          )}
          <div className="space-y-4">
            {files.map((file, index) => (
              <div
                key={index}
                className="border border-gray-200 rounded-md p-3"
              >
                <div className="flex items-start gap-2 mb-2">
                  <div className="flex-1">
                    <input
                      type="text"
                      value={file.path}
                      onChange={(e) =>
                        updateFile(index, "path", e.target.value)
                      }
                      onBlur={() => validateFileField(index, "path")}
                      placeholder="relative/path/to/file.ts"
                      className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 font-mono"
                    />
                    {fieldErrors[`file_path_${index}`] && (
                      <p className="mt-0.5 text-xs text-red-600">
                        {fieldErrors[`file_path_${index}`]}
                      </p>
                    )}
                  </div>
                  <button
                    type="button"
                    onClick={() => removeFile(index)}
                    className="mt-1 text-gray-400 hover:text-red-500 text-sm"
                    aria-label={`Remove file ${file.path || index + 1}`}
                  >
                    ✕
                  </button>
                </div>
                <textarea
                  value={file.content}
                  onChange={(e) =>
                    updateFile(index, "content", e.target.value)
                  }
                  onBlur={() => validateFileField(index, "content")}
                  placeholder="File content..."
                  rows={6}
                  className="block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 resize-y font-mono"
                />
                {fieldErrors[`file_content_${index}`] && (
                  <p className="mt-1 text-xs text-red-600">
                    {fieldErrors[`file_content_${index}`]}
                  </p>
                )}
              </div>
            ))}
          </div>
        </div>

        {/* Submit */}
        <div className="flex items-center gap-3 pt-4 border-t border-gray-200">
          <button
            type="submit"
            disabled={isPending}
            className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50"
          >
            {isPending
              ? isEditMode
                ? "Saving…"
                : "Creating…"
              : isEditMode
                ? "Save Changes"
                : "Create Skill"}
          </button>
          <button
            type="button"
            onClick={() => navigate("/skills")}
            className="inline-flex items-center px-4 py-2 border border-gray-300 text-gray-700 text-sm font-medium rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}
