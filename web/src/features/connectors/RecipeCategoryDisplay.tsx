import { useTranslation } from 'react-i18next';
import { readRecipeCategory } from './recipeCategory';

export default function RecipeCategoryDisplay({ recipe }: { recipe: string }) {
  const { t } = useTranslation();
  const category = readRecipeCategory(recipe);
  return (
    <dl className="mt-2 flex items-baseline gap-2 text-xs">
      <dt className="text-ink-muted">{t('connectors.recipePreview.categoryLabel')}</dt>
      <dd className="text-ink" aria-live="polite">
        {category ? t(`services.category.${category}`) : t('connectors.recipePreview.categoryUnset')}
      </dd>
    </dl>
  );
}
