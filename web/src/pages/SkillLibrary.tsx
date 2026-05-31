import { useState, useMemo, useCallback } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  useSkillTemplates,
  useInstantiateTemplate,
  type SkillTemplateSummary,
} from "../hooks/useSkillTemplates";
import { useSkills } from "../hooks/useSkills";
import { useToast } from "../components/Toast";

/**
 * Skill Library page at `/skills/library`.
 *
 * Displays all available skill templates grouped by category with
 * client-side search filtering. Users can browse, preview, and
 * instantiate templates into their personal skill collection.
 *
 * Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.7, 6.8, 6.9, 10.2
 */
export default function SkillLibrary() {
  const { data: templates, isLoading, isError, refetch } = useSkillTemplates();
  const { data: userSkills } = useSkills();
  const { showToast } = useToast();
  const navigate = useNavigate();
  const instantiate = useInstantiateTemplate();

  const [search, setSearch] = useState("");
  const [instantiatingSlug, setInstantiatingSlug] = useState<string | null>(null);

  // Set of user skill names for "already added" detection
  const userSkillNames = useMemo(() => {
    if (!userSkills) return new Set<string>();
    return new Set(userSkills.map((s) => s.name));
  }, [userSkills]);

  // Filter templates by search term (case-insensitive, name or description)
  const filteredTemplates = useMemo(() => {
    if (!templates) return [];
    if (!search.trim()) return templates;
    const term = search.toLowerCase();
    return templates.filter(
      (t) =>
        t.name.toLowerCase().includes(term) ||
        t.description.toLowerCase().includes(term)
    );
  }, [templates, search]);

  // Group filtered templates by category, sorted alphabetically
  const groupedTemplates = useMemo(() => {
    const groups: Record<string, SkillTemplateSummary[]> = {};
    for (const t of filteredTemplates) {
      if (!groups[t.category]) {
        groups[t.category] = [];
      }
      groups[t.category].push(t);
    }
    // Sort templates within each category by name
    for (const category of Object.keys(groups)) {
      groups[category].sort((a, b) => a.name.localeCompare(b.name));
    }
    // Return sorted category entries
    return Object.entries(groups).sort(([a], [b]) => a.localeCompare(b));
  }, [filteredTemplates]);

  const handleInstantiate = useCallback(
    (slug: string) => {
      setInstantiatingSlug(slug);
      instantiate.mutate(slug, {
        onSuccess: () => {
          showToast("Skill added to your collection", "success");
          setInstantiatingSlug(null);
        },
        onError: (err: Error) => {
          if (err.message?.includes("already")) {
            showToast("This skill name is already in use", "error");
          } else {
            showToast("Failed to add skill. Please try again.", "error");
          }
          setInstantiatingSlug(null);
        },
      });
    },
    [instantiate, showToast]
  );

  return (
    <div className="max-w-7xl mx-auto px-6 py-8">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-xl font-semibold text-gray-900">Skill Library</h2>
        <Link
          to="/skills"
          className="inline-flex items-center px-4 py-2 border border-gray-300 text-gray-700 text-sm font-medium rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
        >
          My Skills
        </Link>
      </div>

      {/* Search */}
      {!isLoading && !isError && templates && templates.length > 0 && (
        <div className="mb-6">
          <input
            type="text"
            placeholder="Search templates by name or description…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full max-w-md px-3 py-2 border border-gray-300 rounded-md text-sm placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          />
        </div>
      )}

      {/* Loading state */}
      {isLoading && <LoadingSkeleton />}

      {/* Error state */}
      {!isLoading && isError && (
        <div className="text-center py-16">
          <div className="text-4xl mb-4">⚠️</div>
          <h3 className="text-lg font-medium text-gray-900 mb-2">
            Failed to load templates
          </h3>
          <p className="text-sm text-gray-500 mb-6">
            Something went wrong while fetching the skill templates.
          </p>
          <button
            onClick={() => refetch()}
            className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
          >
            Retry
          </button>
        </div>
      )}

      {/* Empty search results */}
      {!isLoading &&
        !isError &&
        templates &&
        templates.length > 0 &&
        search.trim() &&
        filteredTemplates.length === 0 && (
          <div className="text-center py-12">
            <p className="text-sm text-gray-500">
              No templates match your search
            </p>
          </div>
        )}

      {/* Grouped template cards */}
      {!isLoading && !isError && groupedTemplates.length > 0 && (
        <div className="space-y-8">
          {groupedTemplates.map(([category, categoryTemplates]) => (
            <section key={category}>
              <h3 className="text-sm font-semibold text-gray-500 uppercase tracking-wide mb-3">
                {category}
              </h3>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {categoryTemplates.map((template) => {
                  const alreadyAdded = userSkillNames.has(template.slug);
                  const isInstantiating = instantiatingSlug === template.slug;

                  return (
                    <TemplateCard
                      key={template.id}
                      template={template}
                      alreadyAdded={alreadyAdded}
                      isInstantiating={isInstantiating}
                      onInstantiate={handleInstantiate}
                      onClick={() => navigate(`/skills/library/${template.slug}`)}
                    />
                  );
                })}
              </div>
            </section>
          ))}
        </div>
      )}
    </div>
  );
}

/* ─── Template Card ─── */

interface TemplateCardProps {
  template: SkillTemplateSummary;
  alreadyAdded: boolean;
  isInstantiating: boolean;
  onInstantiate: (slug: string) => void;
  onClick: () => void;
}

function TemplateCard({
  template,
  alreadyAdded,
  isInstantiating,
  onInstantiate,
  onClick,
}: TemplateCardProps) {
  const descriptionPreview = template.description
    ? template.description.length > 120
      ? template.description.slice(0, 120) + "…"
      : template.description
    : null;

  return (
    <div className="bg-white rounded-lg border border-gray-200 p-4 shadow-sm hover:shadow-md hover:border-gray-300 transition-all flex flex-col">
      {/* Clickable area for navigation */}
      <div className="flex-1 cursor-pointer" onClick={onClick}>
        <div className="flex items-start gap-3 mb-2">
          {template.icon && (
            <span className="text-2xl flex-shrink-0">{template.icon}</span>
          )}
          <div className="min-w-0 flex-1">
            <h4 className="font-medium text-gray-900 truncate">
              {template.name}
            </h4>
            <span className="inline-block text-xs text-gray-400 mt-0.5">
              {template.category}
            </span>
          </div>
        </div>
        {descriptionPreview && (
          <p className="text-sm text-gray-500 mt-1">{descriptionPreview}</p>
        )}
      </div>

      {/* Action button */}
      <div className="mt-3 pt-3 border-t border-gray-100">
        {alreadyAdded ? (
          <span className="inline-flex items-center text-sm text-green-600 font-medium">
            <svg
              className="w-4 h-4 mr-1"
              fill="currentColor"
              viewBox="0 0 20 20"
            >
              <path
                fillRule="evenodd"
                d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                clipRule="evenodd"
              />
            </svg>
            Already Added
          </span>
        ) : (
          <button
            onClick={(e) => {
              e.stopPropagation();
              onInstantiate(template.slug);
            }}
            disabled={isInstantiating}
            className="inline-flex items-center px-3 py-1.5 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isInstantiating ? "Adding…" : "Add to My Skills"}
          </button>
        )}
      </div>
    </div>
  );
}

/* ─── Loading Skeleton ─── */

function LoadingSkeleton() {
  return (
    <div className="space-y-8">
      {Array.from({ length: 2 }).map((_, gi) => (
        <div key={gi}>
          <div className="h-4 w-24 bg-gray-200 rounded mb-3 animate-pulse" />
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {Array.from({ length: 3 }).map((_, i) => (
              <div
                key={i}
                className="bg-white rounded-lg border border-gray-200 p-4 animate-pulse"
              >
                <div className="flex items-start gap-3 mb-2">
                  <div className="h-8 w-8 bg-gray-200 rounded" />
                  <div className="flex-1">
                    <div className="h-5 w-32 bg-gray-200 rounded mb-1" />
                    <div className="h-3 w-16 bg-gray-100 rounded" />
                  </div>
                </div>
                <div className="space-y-2 mt-2">
                  <div className="h-4 w-full bg-gray-100 rounded" />
                  <div className="h-4 w-2/3 bg-gray-100 rounded" />
                </div>
                <div className="mt-3 pt-3 border-t border-gray-100">
                  <div className="h-8 w-32 bg-gray-200 rounded" />
                </div>
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
