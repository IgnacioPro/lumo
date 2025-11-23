import { HTMLAttributes, forwardRef, ReactNode } from 'react';

export interface ContainerProps extends HTMLAttributes<HTMLDivElement> {
  size?: 'sm' | 'md' | 'lg' | 'xl' | 'full';
  children: ReactNode;
}

const Container = forwardRef<HTMLDivElement, ContainerProps>(
  ({ className = '', size = 'lg', children, ...props }, ref) => {
    const sizes = {
      sm: 'max-w-2xl',     // 672px
      md: 'max-w-4xl',     // 896px
      lg: 'max-w-6xl',     // 1152px
      xl: 'max-w-7xl',     // 1280px
      full: 'max-w-full',
    };

    return (
      <div
        ref={ref}
        className={`container mx-auto px-4 ${sizes[size]} ${className}`}
        {...props}
      >
        {children}
      </div>
    );
  }
);

Container.displayName = 'Container';

export default Container;
