import { useMemo } from "react";
import { useParams, Link } from "react-router-dom";
import ReactMarkdown from "react-markdown";
import { useSkillTemplate, useInstantiateTemplate } from "../hooks/useSkillTemplates";
import { useSkills } from "../hooks/useSkills";
import { useToast } from "../components/Toast";

/**
 * Template detail preview page at `/skills/library/:slug`.
 *
 * Displays the full template content rendered as markdown, along with
 * metadata (icon, name, description, category, version). Provides an
 * "Add to My Skills" button or "Already Added" indicator based on
 * whether the user has already instantiated the template.
 *
 * Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 9.6, 9.7
 */
export default function SkillTemplateDetail() {
  const { slug } = useParams<{ slug: string }>();
  const { data: template, isLoading, error } = useSkillTemplate(slug ?? "");
  const { data: skills } = useSkills();
  const instantiate = useInstantiateTemplate();
  const { showToast } = useToast();

  // Detect if user already has a skill with the same name as the template slug
  const alreadyAdded = useMemo(() => {
    if (!skills || !slug) return false;
    return skills.some((skill) => skill.name === slug);
  }, [skills, slug]);

  const handleInstantiate = () => {
    if (!slug) return;
    instantiate.mutate(slug, {
      onSuccess: () => {
        showToast("Skill added to your collection!", "success");
      },
      onError: (err) => {
        const message = err instanceof Error ? err.message : String(err);
        if (message.includes("409") || message.includes("already")) {
          showToast("This skill name is already in use", "error");
        } else {
          showToast("Failed to add skill. Please try again.", "error");
        }
      },
    });
  };

  // Loading state
  if (isLoading) {
    return (
      <div className="max-w-4xl mx-auto px-6 py-8">
        <div className="animate-pulse">
          <div className="h-4 w-32 bg-gray-200 rounded mb-6" />
          <div className="flex items-center gap-4 mb-6">
            <div className="h-12 w-12 bg-gray-200 rounded-lg" />
            <div>
              <div className="h-6 w-48 bg-gray-200 rounded mb-2" />
              <div className="h-4 w-64 bg-gray-100 rounded" />
            </div>
          </div>
          <div className="space-y-3">
            <div className="h-4 w-full bg-gray-100 rounded" />
            <div className="h-4 w-5/6 bg-gray-100 rounded" />
            <div className="h-4 w-4/6 bg-gray-100 rounded" />
          </div>
        </div>
      </div>
    );
  }

  // 404 / error state
  if (error || !template) {
    return (
      <div className="max-w-4xl mx-auto px-6 py-8">
        <div className="text-center py-16">
          <div className="text-4xl mb-4">🔍</div>
          <h3 className="text-lg font-medium text-gray-900 mb-2">
            Template not found
          </h3>
          <p className="text-sm text-gray-500 mb-6">
            The template you&apos;re looking for doesn&apos;t exist or may have been removed.
          </p>
          <Link
            to="/skills/library"
            className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
          >
            ← Back to Library
          </Link>
        </div>
      </div>
    );
  }

  // Determine button state: already added takes precedence over mutation success
  const isAdded = alreadyAdded || instantiate.isSuccess;

  return (
    <div className="max-w-4xl mx-auto px-6 py-8">
      {/* Back to Library link */}
      <Link
        to="/skills/library"
        className="inline-flex items-center text-sm text-gray-500 hover:text-gray-700 mb-6"
      >
        ← Back to Library
      </Link>

      {/* Template header */}
      <div className="flex items-start justify-between mb-8">
        <div className="flex items-center gap-4">
          {template.icon && (
            <span className="text-4xl" aria-hidden="true">
              {template.icon}
            </span>
          )}
          <div>
            <h1 className="text-2xl font-semibold text-gray-900">
              {template.name}
            </h1>
            <p className="text-sm text-gray-500 mt-1">
              {template.description}
            </p>
          </div>
        </div>

        {/* Add / Already Added button */}
        <div className="flex-shrink-0 ml-4">
          {isAdded ? (
            <span className="inline-flex items-center gap-1.5 px-4 py-2 text-sm font-medium text-green-700 bg-green-50 border border-green-200 rounded-md">
              <svg
                className="w-4 h-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
                aria-hidden="true"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M5 13l4 4L19 7"
                />
              </svg>
              Already Added
            </span>
          ) : (
            <button
              onClick={handleInstantiate}
              disabled={instantiate.isPending}
              className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {instantiate.isPending ? "Adding…" : "Add to My Skills"}
            </button>
          )}
        </div>
      </div>

      {/* Metadata badges */}
      <div className="flex items-center gap-3 mb-6">
        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-700">
          {template.category}
        </span>
        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-50 text-blue-700">
          v{template.version}
        </span>
      </div>

      {/* Full markdown content */}
      <div className="bg-white rounded-lg border border-gray-200 shadow-sm p-6">
        <div className="prose prose-sm max-w-none text-gray-700">
          <ReactMarkdown>{template.content}</ReactMarkdown>
        </div>
      </div>
    </div>
  );
}
