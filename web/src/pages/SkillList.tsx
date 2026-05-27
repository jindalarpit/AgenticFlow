import { useState, useMemo } from "react";
import { Link } from "react-router-dom";
import { useSkills, type Skill } from "../hooks/useSkills";

/**
 * Skills list page at `/skills`.
 *
 * Displays all skills owned by the current user with name, description,
 * and agent count. Supports client-side search filtering by name or
 * description (case-insensitive).
 *
 * Requirements: 8.1, 8.2, 8.3, 8.4, 8.5
 */
export default function SkillList() {
  const { data: skills, isLoading } = useSkills();
  const [search, setSearch] = useState("");

  const filteredSkills = useMemo(() => {
    if (!skills) return [];
    if (!search.trim()) return skills;
    const term = search.toLowerCase();
    return skills.filter(
      (skill) =>
        skill.name.toLowerCase().includes(term) ||
        skill.description.toLowerCase().includes(term)
    );
  }, [skills, search]);

  return (
    <div className="max-w-7xl mx-auto px-6 py-8">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-xl font-semibold text-gray-900">Skills</h2>
        <Link
          to="/skills/new"
          className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
        >
          + New Skill
        </Link>
      </div>

      {/* Search */}
      {!isLoading && skills && skills.length > 0 && (
        <div className="mb-6">
          <input
            type="text"
            placeholder="Search skills by name or description…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full max-w-md px-3 py-2 border border-gray-300 rounded-md text-sm placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          />
        </div>
      )}

      {/* Loading state */}
      {isLoading && <LoadingSkeleton />}

      {/* Empty state */}
      {!isLoading && (!skills || skills.length === 0) && <EmptyState />}

      {/* No search matches */}
      {!isLoading &&
        skills &&
        skills.length > 0 &&
        filteredSkills.length === 0 && (
          <div className="text-center py-12">
            <p className="text-sm text-gray-500">
              No skills match &ldquo;{search}&rdquo;
            </p>
          </div>
        )}

      {/* Skills grid */}
      {!isLoading && filteredSkills.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {filteredSkills.map((skill) => (
            <SkillCard key={skill.id} skill={skill} />
          ))}
        </div>
      )}
    </div>
  );
}

/* ─── Skill Card ─── */

function SkillCard({ skill }: { skill: Skill }) {
  const descriptionPreview = skill.description
    ? skill.description.length > 100
      ? skill.description.slice(0, 100) + "…"
      : skill.description
    : null;

  return (
    <Link
      to={`/skills/${skill.id}/edit`}
      className="block bg-white rounded-lg border border-gray-200 p-4 shadow-sm hover:shadow-md hover:border-gray-300 transition-all"
    >
      <div className="flex items-start justify-between mb-2">
        <h3 className="font-medium text-gray-900 truncate">{skill.name}</h3>
        <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-600">
          {skill.agent_count} {skill.agent_count === 1 ? "agent" : "agents"}
        </span>
      </div>

      {descriptionPreview && (
        <p className="text-sm text-gray-500">{descriptionPreview}</p>
      )}
    </Link>
  );
}

/* ─── Loading Skeleton ─── */

function LoadingSkeleton() {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {Array.from({ length: 6 }).map((_, i) => (
        <div
          key={i}
          className="bg-white rounded-lg border border-gray-200 p-4 animate-pulse"
        >
          <div className="flex items-start justify-between mb-2">
            <div className="h-5 w-32 bg-gray-200 rounded" />
            <div className="h-5 w-16 bg-gray-100 rounded" />
          </div>
          <div className="space-y-2">
            <div className="h-4 w-full bg-gray-100 rounded" />
            <div className="h-4 w-2/3 bg-gray-100 rounded" />
          </div>
        </div>
      ))}
    </div>
  );
}

/* ─── Empty State ─── */

function EmptyState() {
  return (
    <div className="text-center py-16">
      <div className="text-4xl mb-4">📚</div>
      <h3 className="text-lg font-medium text-gray-900 mb-2">No skills yet</h3>
      <p className="text-sm text-gray-500 mb-6 max-w-md mx-auto">
        Skills are reusable instruction packages that provide domain knowledge,
        coding standards, or workflow instructions to your agents during task
        execution.
      </p>
      <Link
        to="/skills/new"
        className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
      >
        + Create Skill
      </Link>
    </div>
  );
}
