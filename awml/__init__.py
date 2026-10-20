"""AWML, the Agentic Workflow Markup Language: workflows described as text and drawn with Mermaid."""
from .check import check
from .draw import draw
from .page import page

__all__ = ["check", "draw", "page"]
