import { useCallback, useRef, useState } from 'react';
import { locatedErrorsFrom, type RecipeFeedback, type RecipeFeedbackEntry } from './recipeForm';

type PreviewOutcome = Omit<RecipeFeedbackEntry, 'seq'>;

/**
 * Keeps the save and preview outcomes for a recipe in separate slots, so a late
 * preview cannot overwrite the located errors of a failed save (or the reverse).
 */
export function useRecipeFeedback() {
  const [feedback, setFeedback] = useState<RecipeFeedback>({});
  const submittedRecipe = useRef('');
  const seq = useRef(0);

  const recordSaveStart = useCallback((recipe: string) => {
    submittedRecipe.current = recipe;
    setFeedback((current) => ({ ...current, save: undefined }));
  }, []);

  const recordSaveError = useCallback((error: unknown) => {
    const entry: RecipeFeedbackEntry = { recipe: submittedRecipe.current, errors: locatedErrorsFrom(error), seq: ++seq.current };
    setFeedback((current) => ({ ...current, save: entry }));
  }, []);

  const recordPreview = useCallback((outcome: PreviewOutcome) => {
    const entry: RecipeFeedbackEntry = { ...outcome, seq: ++seq.current };
    setFeedback((current) => ({ ...current, preview: entry }));
  }, []);

  return { feedback, recordSaveStart, recordSaveError, recordPreview };
}
