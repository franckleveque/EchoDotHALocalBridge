prd-generator
Generate comprehensive Product Requirement Documents (PRD) from initial user intentions and accessible documents. Use when the user asks to create, write, or structure a PRD, product specification, or feature requirement document.

Instructions
PRD Generator
This skill guides the AI to analyze inputs, resolve ambiguities, perform all necessary verifications, and draft a complete Product Requirement Document (PRD) with zero unresolved validation gaps.

Workflow Instructions
Phase 1: Context Analysis and Complete Verification
Read the initial intention and analyze all attached/accessible reference documents.
Verify all facts, dependencies, technical constraints, data contracts, and external interfaces mentioned or implied.
Eliminate ambiguities and resolve internal contradictions explicitly.
Mandatory Rule on Verification Completeness: Every statement, interface, or assumption must be fully verified before proceeding to drafting. The presence of phrases like "je n'ai pas vérifié" or "vérification non effectuée" is forbidden and indicates an incomplete task.
Phase 2: Document Structure
Structure the final PRD strictly using the following sections in order:

Vision: Core value proposition, strategic context, and high-level goal.
Interface (contrat externe): Inputs, outputs, APIs, public methods, schemas, or UI/UX boundary contracts.
Concepts (vue d'ensemble): Domain entities, architectural overview, and core workflow definitions.
Fonctionnalités: Detailed functional requirements and operational behavior.
Success Metrics: Measurable KPIs, success criteria, and quantitative goals.
Non-Goals: Explicit out-of-scope items to prevent scope creep.
Risques connus: Operational, technical, legal, or adoption risks with mitigation strategies.
Roadmap: Phased delivery plan and milestones.
MVP: Minimum Viable Product scoping and essential deliverables for launch.
Questions ouvertes: Unresolved edge cases or strategic decisions awaiting stakeholder input.
Glossaire: Key technical, domain, or business terms defined precisely.
Assumptions Index: Catalog of verified and active working assumptions.
Guidelines and Formatting Rules
Use clear, professional, and precise language.
Maintain consistency across all sections.
Ensure every assumption listed in the Assumptions Index maps directly to functional rules or system bounds.
